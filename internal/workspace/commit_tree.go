package workspace

import (
	"bytes"
	"context"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
)

type stagedLeaf struct {
	mode string
	oid  string
	path string
}

func (s *Service) readFullIndexTree(ctx context.Context, cwd string) (digest, treeOID string, hasUnmerged bool, err error) {
	data, err := ReadGit(ctx, cwd, maxWorkspaceGitOutput, "ls-files", "--stage", "-z")
	if err != nil {
		return "", "", false, err
	}
	sum := sha256.Sum256(data)
	digest = hex.EncodeToString(sum[:])
	if len(data) == 0 {
		empty, eErr := s.emptyTreeOID(ctx, cwd)
		return digest, empty, false, eErr
	}

	leaves := make([]stagedLeaf, 0)
	for _, rec := range bytes.Split(bytes.TrimSuffix(data, []byte{0}), []byte{0}) {
		if len(rec) == 0 {
			continue
		}
		meta, relPath, ok := strings.Cut(string(rec), "\t")
		if !ok {
			continue
		}
		parts := strings.Fields(meta)
		if len(parts) != 3 {
			continue
		}
		if parts[2] != "0" {
			hasUnmerged = true
			continue
		}
		leaves = append(leaves, stagedLeaf{mode: parts[0], oid: parts[1], path: relPath})
	}
	if hasUnmerged {
		return digest, "", true, nil
	}
	if len(leaves) == 0 {
		empty, eErr := s.emptyTreeOID(ctx, cwd)
		return digest, empty, false, eErr
	}
	computed, err := hashGitTreeFromLeaves(leaves)
	if err != nil {
		return "", "", false, err
	}
	return digest, computed, false, nil
}

func hashGitTreeFromLeaves(leaves []stagedLeaf) (string, error) {
	type nodeEntry struct {
		name  string
		isDir bool
		mode  string
		raw   []byte
	}
	useSHA256 := len(leaves[0].oid) == 64

	var build func(items []stagedLeaf) ([]byte, error)
	build = func(items []stagedLeaf) ([]byte, error) {
		entries := make([]nodeEntry, 0)
		subdirs := make(map[string][]stagedLeaf)
		for _, item := range items {
			head, tail, hasSub := strings.Cut(item.path, "/")
			if !hasSub {
				rawHash, err := hex.DecodeString(item.oid)
				if err != nil {
					return nil, problemf(ErrInternal, "无效的索引对象 ID: %s", item.oid)
				}
				entries = append(entries, nodeEntry{
					name:  head,
					isDir: false,
					mode:  strings.TrimLeft(item.mode, "0"),
					raw:   rawHash,
				})
			} else {
				subdirs[head] = append(subdirs[head], stagedLeaf{mode: item.mode, oid: item.oid, path: tail})
			}
		}
		for dirName, children := range subdirs {
			subRaw, err := build(children)
			if err != nil {
				return nil, err
			}
			entries = append(entries, nodeEntry{
				name:  dirName,
				isDir: true,
				mode:  "40000",
				raw:   subRaw,
			})
		}
		sort.Slice(entries, func(i, j int) bool {
			ki := entries[i].name
			if entries[i].isDir {
				ki += "/"
			}
			kj := entries[j].name
			if entries[j].isDir {
				kj += "/"
			}
			return ki < kj
		})
		var body bytes.Buffer
		for _, e := range entries {
			_, _ = fmt.Fprintf(&body, "%s %s\x00", e.mode, e.name)
			_, _ = body.Write(e.raw)
		}
		var full bytes.Buffer
		_, _ = fmt.Fprintf(&full, "tree %d\x00", body.Len())
		_, _ = full.Write(body.Bytes())
		if useSHA256 {
			sum := sha256.Sum256(full.Bytes())
			return sum[:], nil
		}
		sum := sha1.Sum(full.Bytes())
		return sum[:], nil
	}

	rootRaw, err := build(leaves)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(rootRaw), nil
}
