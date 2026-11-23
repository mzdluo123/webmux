// Package agentcheck inspects locally installed CLI agent tools and their
// external ACP connectors, caches local scan snapshots in memory, and queries
// read-only public package registries on explicit update-check requests.
package agentcheck

import (
	"context"
	"maps"
	"net/http"
	"os"
	"slices"
	"sync"
	"time"

	"github.com/rainchan/webmux/internal/agentcatalog"
	"github.com/rainchan/webmux/internal/agentprofile"
)

// UpdateStatus describes the version check state of one tool or connector.
type UpdateStatus string

const (
	StatusNotInstalled      UpdateStatus = "not_installed"
	StatusUnchecked         UpdateStatus = "unchecked"
	StatusUpToDate          UpdateStatus = "up_to_date"
	StatusUpdateAvailable   UpdateStatus = "update_available"
	StatusSourceUnavailable UpdateStatus = "source_unavailable"
	StatusCheckFailed       UpdateStatus = "check_failed"
)

const (
	defaultLocalTTL       = 30 * time.Second
	defaultRemoteTTL      = 30 * time.Minute
	defaultLookupTimeout  = 5 * time.Second
	defaultVersionTimeout = 1500 * time.Millisecond
)

// ConnectorStatus reports the local installation and upstream update state of
// an external ACP connector binary.
type ConnectorStatus struct {
	Executable       string
	Title            string
	Installed        bool
	Path             string
	InstalledVersion string
	LatestVersion    string
	VerifiedVersion  string
	BelowVerified    bool
	UpdateStatus     UpdateStatus
	CheckError       string
	InstallCommand   string
	UpgradeCommand   string
	DocsURL          string
}

// ToolStatus reports the local installation and upstream update state of one
// built-in CLI agent tool.
type ToolStatus struct {
	ID               string
	Title            string
	Icon             string
	Kind             agentcatalog.Kind
	Installed        bool
	Executable       string
	Executables      []string
	Path             string
	InstalledVersion string
	LatestVersion    string
	VerifiedVersion  string
	BelowVerified    bool
	UpdateStatus     UpdateStatus
	CheckError       string
	InstallCommand   string
	UpgradeCommand   string
	DocsURL          string
	Connector        *ConnectorStatus
}

// Report is a point-in-time snapshot of all cataloged CLI agent tools.
type Report struct {
	CheckedAt       time.Time
	RemoteCheckedAt *time.Time
	Shell           string
	Tools           []ToolStatus
}

// Options configures a Service instance. Zero values select safe defaults.
type Options struct {
	Entries        []agentcatalog.Entry
	ShellPath      string
	Env            []string
	HTTPClient     *http.Client
	NPMBases       []string
	PyPIBase       string
	GitHubAPIBase  string
	LocalTTL       time.Duration
	RemoteTTL      time.Duration
	LookupTimeout  time.Duration
	VersionTimeout time.Duration
	LocalBudget    time.Duration
	RemoteBudget   time.Duration
	Now            func() time.Time
}

type flight[T any] struct {
	done chan struct{}
	val  T
	err  error
}

// Service inspects local CLI agent installations and checks public registries
// on explicit user demand. All caches are kept strictly in memory.
type Service struct {
	entries              []agentcatalog.Entry
	shellPath            string
	env                  []string
	client               *http.Client
	npmBases             []string
	pypiBase             string
	githubAPIBase        string
	allowedHosts         map[string]string
	localTTL             time.Duration
	remoteTTL            time.Duration
	lookupTimeout        time.Duration
	versionTimeout       time.Duration
	localBudgetOverride  time.Duration
	remoteBudgetOverride time.Duration
	now                  func() time.Time

	mu               sync.Mutex
	localCached      *localSnapshot
	localFlight      *flight[localSnapshot]
	versionCache     map[string]cachedBinaryVersion
	versionReprobing map[string]bool
	remoteCached     map[agentprofile.UpstreamSource]remoteEntry
	remoteCheckedAt  *time.Time
	remoteFlight     *flight[struct{}]
}

// New constructs an in-memory agent tool inspection service.
func New(opts Options) *Service {
	npmBases := slices.Clone(opts.NPMBases)
	if len(npmBases) == 0 {
		npmBases = slices.Clone(defaultNPMBases)
	}
	pypiBase := opts.PyPIBase
	if pypiBase == "" {
		pypiBase = defaultPyPIBase
	}
	githubBase := opts.GitHubAPIBase
	if githubBase == "" {
		githubBase = defaultGitHubAPIBase
	}
	localTTL := opts.LocalTTL
	if localTTL <= 0 {
		localTTL = defaultLocalTTL
	}
	remoteTTL := opts.RemoteTTL
	if remoteTTL <= 0 {
		remoteTTL = defaultRemoteTTL
	}
	lookupTimeout := opts.LookupTimeout
	if lookupTimeout <= 0 {
		lookupTimeout = defaultLookupTimeout
	}
	versionTimeout := opts.VersionTimeout
	if versionTimeout <= 0 {
		versionTimeout = defaultVersionTimeout
	}
	nowFn := opts.Now
	if nowFn == nil {
		nowFn = time.Now
	}

	allBases := make([]string, 0, len(npmBases)+2)
	allBases = append(allBases, npmBases...)
	allBases = append(allBases, pypiBase, githubBase)
	allowed := buildAllowedHosts(allBases...)

	return &Service{
		entries:              cloneEntries(opts.Entries),
		shellPath:            opts.ShellPath,
		env:                  slices.Clone(opts.Env),
		client:               newHardenedClient(opts.HTTPClient, defaultUpstreamTimeout, allowed),
		npmBases:             npmBases,
		pypiBase:             pypiBase,
		githubAPIBase:        githubBase,
		allowedHosts:         allowed,
		localTTL:             localTTL,
		remoteTTL:            remoteTTL,
		lookupTimeout:        lookupTimeout,
		versionTimeout:       versionTimeout,
		localBudgetOverride:  opts.LocalBudget,
		remoteBudgetOverride: opts.RemoteBudget,
		now:                  nowFn,
		versionCache:         make(map[string]cachedBinaryVersion),
		versionReprobing:     make(map[string]bool),
		remoteCached:         make(map[agentprofile.UpstreamSource]remoteEntry),
	}
}

func cloneEntries(entries []agentcatalog.Entry) []agentcatalog.Entry {
	out := make([]agentcatalog.Entry, len(entries))
	for i, e := range entries {
		e.Executables = slices.Clone(e.Executables)
		if e.Connector != nil {
			c := *e.Connector
			e.Connector = &c
		}
		out[i] = e
	}
	return out
}

// Inspect returns the local agent tool installation report merged with any
// unexpired in-memory upstream check results. It never performs outbound
// network requests.
func (s *Service) Inspect(ctx context.Context, refresh bool) (Report, error) {
	snap, err := s.getLocalSnapshot(ctx, refresh)
	if err != nil {
		return Report{}, err
	}
	return s.buildReport(snap), nil
}

// CheckUpdates scans local tools (reusing the local cache unless refreshLocal
// is true), queries public upstream registries for installed tools and
// connectors, caches the results in memory, and returns the combined report.
func (s *Service) CheckUpdates(ctx context.Context, refreshLocal bool) (Report, error) {
	snap, err := s.getLocalSnapshot(ctx, refreshLocal)
	if err != nil {
		return Report{}, err
	}
	if err := s.runRemoteCheck(ctx, snap); err != nil {
		return Report{}, err
	}
	return s.buildReport(snap), nil
}

func (s *Service) getLocalSnapshot(ctx context.Context, refresh bool) (localSnapshot, error) {
	if err := ctx.Err(); err != nil {
		return localSnapshot{}, err
	}
	if refresh {
		snap, err := s.scanHost(ctx, false, true)
		if err == nil {
			s.mu.Lock()
			s.localCached = &snap
			s.mu.Unlock()
		}
		return snap, err
	}

	s.mu.Lock()
	if s.localCached != nil && s.now().Sub(s.localCached.checkedAt) < s.localTTL {
		snap := *s.localCached
		s.mu.Unlock()
		return snap, nil
	}
	f := s.localFlight
	if f == nil {
		f = &flight[localSnapshot]{done: make(chan struct{})}
		s.localFlight = f
		s.mu.Unlock()
		go s.finishLocalFlight(ctx, f)
	} else {
		s.mu.Unlock()
	}

	select {
	case <-f.done:
		return f.val, f.err
	case <-ctx.Done():
		return localSnapshot{}, ctx.Err()
	}
}

func (s *Service) localBudget() time.Duration {
	if s.localBudgetOverride > 0 {
		return s.localBudgetOverride
	}
	candidates := len(collectCandidates(s.entries))
	waves := max(1, (candidates+versionConcurrency-1)/versionConcurrency)
	return s.lookupTimeout + time.Duration(waves)*s.versionTimeout + 5*time.Second
}

func (s *Service) finishLocalFlight(parent context.Context, f *flight[localSnapshot]) {
	flightCtx, cancel := context.WithTimeout(context.WithoutCancel(parent), s.localBudget())
	defer cancel()

	snap, err := s.scanHost(flightCtx, true, false)

	s.mu.Lock()
	if err == nil {
		s.localCached = &snap
	}
	if s.localFlight == f {
		s.localFlight = nil
	}
	f.val, f.err = snap, err
	close(f.done)
	s.mu.Unlock()
}

func (s *Service) runRemoteCheck(ctx context.Context, snap localSnapshot) error {
	for attempt := range 2 {
		if err := ctx.Err(); err != nil {
			return err
		}
		s.mu.Lock()
		f := s.remoteFlight
		joined := f != nil
		if !joined {
			f = &flight[struct{}]{done: make(chan struct{})}
			s.remoteFlight = f
			s.mu.Unlock()
			go s.finishRemoteFlight(ctx, snap, f)
		} else {
			s.mu.Unlock()
		}

		select {
		case <-f.done:
			if f.err != nil || !joined || attempt > 0 || s.hasCachedUpstreams(snap) {
				return f.err
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

func (s *Service) hasCachedUpstreams(snap localSnapshot) bool {
	sources := collectInstalledUpstreams(snap)
	if len(sources) == 0 {
		return true
	}
	now := s.now()
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, u := range sources {
		rec, ok := s.remoteCached[u]
		if !ok || now.Sub(rec.checkedAt) >= s.remoteTTL {
			return false
		}
	}
	return true
}

func (s *Service) remoteBudgetFor(sources []agentprofile.UpstreamSource) time.Duration {
	if s.remoteBudgetOverride > 0 {
		return s.remoteBudgetOverride
	}
	waves := max(1, (len(sources)+upstreamConcurrency-1)/upstreamConcurrency)
	npmAttempts := max(1, len(s.npmBases))
	perAttempt := max(s.client.Timeout, time.Second)
	return time.Duration(waves)*time.Duration(npmAttempts)*perAttempt + 5*time.Second
}

func (s *Service) finishRemoteFlight(parent context.Context, snap localSnapshot, f *flight[struct{}]) {
	sources := collectInstalledUpstreams(snap)
	flightCtx, cancel := context.WithTimeout(context.WithoutCancel(parent), s.remoteBudgetFor(sources))
	defer cancel()

	fetched := s.fetchUpstreams(flightCtx, sources)

	s.mu.Lock()
	now := s.now()
	maps.Copy(s.remoteCached, fetched)
	s.remoteCheckedAt = &now
	if s.remoteFlight == f {
		s.remoteFlight = nil
	}
	f.err = nil
	close(f.done)
	s.mu.Unlock()
}

func (s *Service) buildReport(snap localSnapshot) Report {
	now := s.now()
	s.mu.Lock()
	remote := make(map[agentprofile.UpstreamSource]remoteEntry, len(s.remoteCached))
	for k, v := range s.remoteCached {
		if now.Sub(v.checkedAt) < s.remoteTTL {
			remote[k] = v
		}
	}
	var remoteCheckedAt *time.Time
	if s.remoteCheckedAt != nil && now.Sub(*s.remoteCheckedAt) < s.remoteTTL {
		ts := *s.remoteCheckedAt
		remoteCheckedAt = &ts
	}
	s.mu.Unlock()

	tools := make([]ToolStatus, 0, len(snap.tools))
	for _, lt := range snap.tools {
		e := lt.entry
		status, latest, checkErr := evaluateStatus(lt.installed, lt.installedVersion, e.Distribution.Upstream, remote)
		var conn *ConnectorStatus
		if lt.connector != nil {
			lc := lt.connector
			ce := lc.entry
			cStatus, cLatest, cErr := evaluateStatus(lc.installed, lc.installedVersion, ce.Distribution.Upstream, remote)
			conn = &ConnectorStatus{
				Executable:       ce.Executable,
				Title:            ce.Title,
				Installed:        lc.installed,
				Path:             lc.path,
				InstalledVersion: lc.installedVersion,
				LatestVersion:    cLatest,
				VerifiedVersion:  ce.VerifiedVersion,
				BelowVerified:    lc.belowVerified,
				UpdateStatus:     cStatus,
				CheckError:       cErr,
				InstallCommand:   ce.Distribution.InstallCommand,
				UpgradeCommand:   ce.Distribution.UpgradeCommand,
				DocsURL:          ce.Distribution.DocsURL,
			}
		}
		tools = append(tools, ToolStatus{
			ID:               e.ID,
			Title:            e.Title,
			Icon:             e.Icon,
			Kind:             e.Kind,
			Installed:        lt.installed,
			Executable:       lt.executable,
			Executables:      slices.Clone(e.Executables),
			Path:             lt.path,
			InstalledVersion: lt.installedVersion,
			LatestVersion:    latest,
			VerifiedVersion:  e.VerifiedVersion,
			BelowVerified:    lt.belowVerified,
			UpdateStatus:     status,
			CheckError:       checkErr,
			InstallCommand:   e.Distribution.InstallCommand,
			UpgradeCommand:   e.Distribution.UpgradeCommand,
			DocsURL:          e.Distribution.DocsURL,
			Connector:        conn,
		})
	}
	return Report{
		CheckedAt:       snap.checkedAt,
		RemoteCheckedAt: remoteCheckedAt,
		Shell:           snap.shell,
		Tools:           tools,
	}
}

func evaluateStatus(installed bool, installedVer string, upstream agentprofile.UpstreamSource, remote map[agentprofile.UpstreamSource]remoteEntry) (status UpdateStatus, latest, checkErr string) {
	if !installed {
		return StatusNotInstalled, "", ""
	}
	if upstream.Kind == agentprofile.UpstreamNone || upstream.Package == "" {
		return StatusSourceUnavailable, "", ""
	}
	rec, ok := remote[upstream]
	if !ok {
		return StatusUnchecked, "", ""
	}
	if rec.err != "" {
		return StatusCheckFailed, "", rec.err
	}
	latest = rec.version
	order, valid := agentprofile.CompareVersions(installedVer, latest)
	if !valid {
		return StatusCheckFailed, latest, "无法解析本地版本号"
	}
	if order < 0 {
		return StatusUpdateAvailable, latest, ""
	}
	return StatusUpToDate, latest, ""
}

func (s *Service) baseEnv() []string {
	if len(s.env) > 0 {
		return slices.Clone(s.env)
	}
	return os.Environ()
}

func (s *Service) shellName(env []string) string {
	if s.shellPath != "" {
		return s.shellPath
	}
	return envLookup(env, "SHELL")
}
