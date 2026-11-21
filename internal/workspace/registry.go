package workspace

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"github.com/rainchan/webmux/internal/config"
	"github.com/rainchan/webmux/internal/hybrid"
	"github.com/rainchan/webmux/internal/launch"
	"github.com/rainchan/webmux/internal/pathutil"
	"github.com/rainchan/webmux/internal/session"
	"github.com/rainchan/webmux/internal/statefile"
)

// MaxInitInstruction is the most characters a repository's initialization
// instruction may have.
const MaxInitInstruction = 4000

// MaxRepositories bounds repositories.json, like the other stores.
const MaxRepositories = 500

type registration struct {
	ID          string `json:"id"`
	CommonDir   string `json:"commonDir"`
	AnchorPath  string `json:"anchorPath"`
	DisplayName string `json:"displayName"`
	// InitInstruction prefixes a quick start's first prompt. Files written
	// before it existed lack it: no instruction.
	InitInstruction string   `json:"initInstruction,omitempty"`
	KnownWorktrees  []string `json:"knownWorktrees,omitempty"`
	KnownSeeded     bool     `json:"knownSeeded,omitempty"`
}
type registry struct {
	Version      int            `json:"version"`
	Repositories []registration `json:"repositories"`
}
type repositoryState struct {
	// entry is immutable; its mutable fields stay zero and live on
	// repositoryState under Service.mu.
	entry registration
	// instruction, knownSeeded and knownWorktrees are guarded by Service.mu.
	instruction    string
	knownSeeded    bool
	knownWorktrees []string
	gate           chan struct{}
	flightMu       sync.Mutex
	flight         *snapshotFlight
	// statusCache holds per-worktree status entries reused across List calls for
	// worktrees whose Git/index/ref metadata has not changed.
	statusCacheMu sync.Mutex
	statusCache   map[string]cachedWorktreeStatus
	// snapMu guards the last completed repository snapshot for opt-in SWR reads.
	snapMu          sync.Mutex
	lastSnapshot    Repository
	hasSnapshot     bool
	lastSnapshotAt  time.Time
	lastFingerprint uint64
	snapGen         uint64
	snapDirty       bool
	revalidating    bool
	// fetching is the running FetchRemotes; guarded by fetchMu.
	fetchMu  sync.Mutex
	fetching *fetchFlight
	// swept is set once a sweep of orphan release refs ran; guarded by
	// gate.
	swept bool
}
type snapshotFlight struct {
	done       chan struct{}
	repository Repository
	err        error
	cancel     context.CancelFunc
	waiters    int
	cached     bool
}
type Service struct {
	projectMu     sync.Mutex
	projectSaveMu sync.Mutex
	projectStore  *statefile.Handle[projectRegistry]
	projects      []directoryProject
	projectError  error
	// mkdirProject is a per-service filesystem port; nil uses os.Mkdir.
	mkdirProject func(string) error
	cloneMu      sync.Mutex
	cloneSaveMu  sync.Mutex
	cloneStore   *statefile.Handle[projectCloneRegistry]
	clones       []projectCloneRecord
	cloneError   error
	cloneContext context.Context
	cloneCancel  context.CancelFunc
	cloneWG      sync.WaitGroup
	cloneSlots   chan struct{}
	cloneStopped bool

	manager  *session.Manager
	launches *launch.Service
	hybrids  *hybrid.Service
	// commands lists the saved quick commands an initialization instruction
	// may reference; nil means none exist.
	commands                   func() []QuickCommand
	pinHomeWorktrees           func(repoID, mainWorktreeID string, added []string) error
	retainWorktreePins         func(repoID string, liveIDs []string) error
	dropRepositoryWorktreePins func(repoID string) error
	defaultCwd                 string
	gitPath                    string
	githubPath                 string
	githubTimeout              time.Duration
	fetchTimeout               time.Duration
	// fetchRemoteTimeout bounds each remote of FetchRemotes.
	fetchRemoteTimeout time.Duration
	stateDir           string
	// worktreeGate bounds concurrent worktree status reads across every
	// repository in a list, not just within one repository.
	worktreeGate          chan struct{}
	writeGatesMu          sync.Mutex
	writeGates            map[string]chan struct{}
	mu                    sync.Mutex
	associations          map[string]Association
	upstreamConfirmations map[string]upstreamConfirmation
	// associationMu orders cache commits through their source publication.
	// Capture reads only associationCacheMu, never registry or process state.
	associationMu      sync.Mutex
	associationCacheMu sync.RWMutex
	stopAssociations   func()
	// saveMu serializes registry read-modify-write cycles without holding mu
	// across the state file commit.
	saveMu        sync.Mutex
	capability    Capability
	entries       []*repositoryState
	store         *statefile.Handle[registry]
	coord         sync.Mutex
	starts        map[string]int
	reservations  map[string]bool
	changed       chan struct{}
	confirmations map[string]confirmation

	opsMu        sync.Mutex
	opsSaveMu    sync.Mutex
	opsStore     *statefile.Handle[gitOperationLedger]
	opsAvailable bool
	opsError     string
	operations   []GitOperation
	opFlights    map[string]*opFlight

	baselinesMu        sync.Mutex
	baselinesSaveMu    sync.Mutex
	baselinesStore     *statefile.Handle[taskBaselineStore]
	baselinesAvailable bool
	baselinesError     string
	baselines          []TaskBaseline

	collabMu        sync.Mutex
	collabSaveMu    sync.Mutex
	collabStore     *statefile.Handle[gitWorkbenchStateStore]
	collabAvailable bool
	collabError     string
	collabDrafts    []commitDraftRecord
	collabReviews   []ReviewMarkRecord

	watchMu       sync.Mutex
	watchSubs     map[*Subscription]struct{}
	watchStop     context.CancelFunc
	watchDone     chan struct{}
	watchInterval time.Duration
	watchDebounce time.Duration
	closed        bool

	swrMu     sync.Mutex
	swrCtx    context.Context
	swrCancel context.CancelFunc
	swrWG     sync.WaitGroup
	swrClosed bool
	freshTTL  time.Duration
}

// New never makes optional Git or storage availability a server startup failure.
// launches resolves saved launch configs; nil makes them unavailable.
func New(stateDir string, manager *session.Manager, launches *launch.Service, defaultCwd string) *Service {
	if stateDir == "" {
		stateDir = config.DefaultStateDir()
	}
	swrCtx, swrCancel := context.WithCancel(context.Background())
	s := &Service{manager: manager, launches: launches, defaultCwd: defaultCwd, stateDir: stateDir, fetchTimeout: branchFetchTimeout, fetchRemoteTimeout: fetchRemoteTimeout, worktreeGate: make(chan struct{}, worktreeConcurrency()), writeGates: make(map[string]chan struct{}), starts: make(map[string]int), reservations: make(map[string]bool), changed: make(chan struct{}), confirmations: make(map[string]confirmation), opFlights: make(map[string]*opFlight), watchInterval: defaultWatchInterval, watchDebounce: defaultWatchDebounce, swrCtx: swrCtx, swrCancel: swrCancel, freshTTL: defaultSnapshotFreshTTL}
	s.githubPath, _ = exec.LookPath("gh")
	s.githubTimeout = githubRequestTimeout
	git, err := exec.LookPath("git")
	if err != nil {
		s.capability.GitError = "Git 不可用: " + err.Error()
	} else {
		s.gitPath = git
		s.capability.GitAvailable = true
	}
	if err := s.load(); err != nil {
		s.capability.RegistryError = "仓库清单不可用: " + err.Error()
	} else {
		s.capability.RegistryAvailable = true
	}
	s.loadProjects()
	s.loadProjectClones()
	if err := s.loadOperations(); err != nil {
		s.opsError = "Git 操作账本不可用: " + err.Error()
	} else {
		s.opsAvailable = true
	}
	if err := s.loadTaskBaselines(); err != nil {
		s.baselinesError = "Git 任务基线不可用: " + err.Error()
	} else {
		s.baselinesAvailable = true
	}
	if err := s.loadWorkbenchCollab(); err != nil {
		s.collabError = "Git 工作台协作状态不可用: " + err.Error()
	} else {
		s.collabAvailable = true
	}
	if s.opsAvailable {
		s.reconcileOperationsOnStartup()
	}
	if manager != nil {
		s.stopAssociations = manager.Observe(associationCleanup{service: s})
	}
	return s
}
func (s *Service) Close() {
	s.closeWatcher()
	s.closeSWR()
	s.closeProjectClones()
	s.closeProjects()
	if s.stopAssociations != nil {
		s.stopAssociations()
	}
	s.mu.Lock()
	s.store.Close()
	s.capability.RegistryAvailable = false
	s.capability.RegistryError = "仓库服务已关闭"
	s.clearConfirmations()
	s.mu.Unlock()
	s.closeOperations()
	s.closeTaskBaselines()
	s.closeWorkbenchCollab()
}
func (s *Service) Capability() Capability { s.mu.Lock(); defer s.mu.Unlock(); return s.capability }

// UseHybrid installs, once at assembly before serving, the service that
// starts sessions of hybrid launch configs. Without it they answer
// ErrUnavailable.
func (s *Service) UseHybrid(h *hybrid.Service) { s.hybrids = h }

// UseQuickCommands installs, once at assembly before serving, the source of
// the saved quick commands that initialization instructions reference by
// name.
func (s *Service) UseQuickCommands(list func() []QuickCommand) { s.commands = list }

func (s *Service) load() error {
	expanded, err := pathutil.Expand(s.stateDir)
	if err != nil {
		return err
	}
	s.stateDir = expanded
	var stored registry
	s.store, stored, err = statefile.Open[registry](s.stateDir, statefile.Repositories, validateRegistry)
	if statefile.IsKind(err, statefile.Missing) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, entry := range stored.Repositories {
		s.entries = append(s.entries, newRepository(entry))
	}
	return nil
}
func validateRegistry(stored registry) error {
	if stored.Version != 1 || stored.Repositories == nil {
		return errors.New("不支持或损坏的仓库清单")
	}
	if len(stored.Repositories) > MaxRepositories {
		return fmt.Errorf("仓库清单超过 %d 个仓库", MaxRepositories)
	}
	ids, paths := map[string]bool{}, map[string]bool{}
	for _, entry := range stored.Repositories {
		if entry.ID == "" || !filepath.IsAbs(entry.CommonDir) || !filepath.IsAbs(entry.AnchorPath) || ids[entry.ID] || paths[entry.CommonDir] {
			return errors.New("仓库清单身份无效或重复")
		}
		if len(entry.KnownWorktrees) > maxKnownWorktreesPerRepo {
			return errors.New("仓库已知工作树数量超过上限")
		}
		seenKnown := make(map[string]bool, len(entry.KnownWorktrees))
		for _, wid := range entry.KnownWorktrees {
			if !validHomeWorktreeID(wid) || seenKnown[wid] {
				return errors.New("仓库已知工作树标识无效或重复")
			}
			seenKnown[wid] = true
		}
		ids[entry.ID], paths[entry.CommonDir] = true, true
	}
	return nil
}
func newRepository(entry registration) *repositoryState {
	r := &repositoryState{
		entry:          entry,
		instruction:    entry.InitInstruction,
		knownSeeded:    entry.KnownSeeded,
		knownWorktrees: slices.Clone(entry.KnownWorktrees),
		gate:           make(chan struct{}, 1),
	}
	r.entry.InitInstruction = ""
	r.entry.KnownSeeded = false
	r.entry.KnownWorktrees = nil
	r.gate <- struct{}{}
	return r
}

func (r *repositoryState) registrationLocked() registration {
	item := r.entry
	item.InitInstruction = r.instruction
	item.KnownSeeded = r.knownSeeded
	if len(r.knownWorktrees) > 0 {
		item.KnownWorktrees = slices.Clone(r.knownWorktrees)
	}
	return item
}

// initInstruction is r's current initialization instruction.
func (s *Service) initInstruction(r *repositoryState) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return r.instruction
}

func (s *Service) save(entries []*repositoryState) error {
	return s.saveWith(entries, nil, "")
}

// saveWith writes entries, with instruction in place of changed's.
func (s *Service) saveWith(entries []*repositoryState, changed *repositoryState, instruction string) error {
	stored := registry{Version: 1, Repositories: make([]registration, 0, len(entries))}
	s.mu.Lock()
	for _, entry := range entries {
		item := entry.registrationLocked()
		if entry == changed {
			item.InitInstruction = instruction
		}
		stored.Repositories = append(stored.Repositories, item)
	}
	s.mu.Unlock()
	return s.commitRegistry(stored)
}

func (s *Service) commitRegistry(stored registry) error {
	err := s.store.Commit(stored)
	if statefile.IsKind(err, statefile.CommitUncertain) {
		// Rename already published different bytes. Keep the last confirmed
		// memory state, but never allow it to overwrite the uncertain disk state.
		message := "清单已替换但目录同步失败，持久结果不确定；仓库管理已停用，请重启重新读取: " + err.Error()
		s.mu.Lock()
		s.capability.RegistryAvailable = false
		s.capability.RegistryError = message
		s.mu.Unlock()
		return errors.New(message)
	}
	return err
}

func (s *Service) requireRegistryLocked() error {
	if !s.capability.RegistryAvailable {
		return problemf(ErrUnavailable, "%s", s.capability.RegistryError)
	}
	return nil
}
func (s *Service) repository(id string) (*repositoryState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.requireRegistryLocked(); err != nil {
		return nil, err
	}
	for _, r := range s.entries {
		if r.entry.ID == id {
			return r, nil
		}
	}
	return nil, problemf(ErrNotFound, "仓库不存在，请刷新")
}
func (s *Service) registered(r *repositoryState) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, candidate := range s.entries {
		if candidate == r {
			return true
		}
	}
	return false
}

func (s *Service) requireRegistry() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.requireRegistryLocked()
}
func (s *Service) Unregister(id string) error {
	r, err := s.repository(id)
	if err != nil {
		return err
	}
	s.saveMu.Lock()
	defer s.saveMu.Unlock()
	s.mu.Lock()
	if err := s.requireRegistryLocked(); err != nil {
		s.mu.Unlock()
		return err
	}
	next := make([]*repositoryState, 0, len(s.entries))
	found := false
	for _, item := range s.entries {
		if item == r {
			found = true
		} else {
			next = append(next, item)
		}
	}
	if !found {
		s.mu.Unlock()
		return problemf(ErrNotFound, "仓库不存在")
	}
	s.mu.Unlock()
	if err := s.save(next); err != nil {
		return fmt.Errorf("保存仓库清单: %w", err)
	}
	s.mu.Lock()
	s.entries = next
	s.mu.Unlock()
	s.purgeRepositoryOperations(id)
	s.purgeRepositoryBaselines(r, id)
	s.purgeRepositoryCollabState(id)
	s.purgeRepositoryWorktreePins(id)
	for _, info := range s.manager.List() {
		if association := s.Association(info.ID); association != nil && association.RepositoryID == id {
			s.commitAssociation(info, nil, false)
		}
	}
	s.notifyWorkspaces()
	return nil
}
func (s *Service) Refresh(ctx context.Context, id string) (Repository, error) {
	r, err := s.repository(id)
	if err != nil {
		return Repository{}, err
	}
	snapshot, err := s.snapshot(ctx, r)
	if err == nil {
		s.reassociate(ctx)
	}
	return snapshot, err
}
func acquire(ctx context.Context, r *repositoryState) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-r.gate:
		return nil
	}
}
func release(r *repositoryState) { r.gate <- struct{}{} }
