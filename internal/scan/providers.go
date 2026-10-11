package scan

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Asymtode712/devsweep/internal/core"
	"github.com/Asymtode712/devsweep/internal/rules"
)

// huggingFaceItems lists each cached repo in the hub cache as its own item.
// Snapshots are symlinks into blobs/, which the sizer never follows, so a
// repo's size is its blobs counted once.
func huggingFaceItems(env *rules.Env) []*core.Item {
	r := rules.ByID("ai.huggingface")
	hub := rules.HFHubDir(env)
	entries, err := os.ReadDir(hub)
	if err != nil {
		return nil
	}
	var out []*core.Item
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		kind, repo, ok := strings.Cut(e.Name(), "--")
		if !ok || (kind != "models" && kind != "datasets" && kind != "spaces") {
			continue
		}
		repo = strings.ReplaceAll(repo, "--", "/")
		path := filepath.Join(hub, e.Name())
		name, restore := repo, "huggingface-cli download "+repo
		if kind != "models" {
			name = kind[:len(kind)-1] + ": " + repo
			restore += " --repo-type " + kind[:len(kind)-1]
		}
		out = append(out, &core.Item{
			ID: "hf:" + path, RuleID: r.ID, Ecosystem: r.Ecosystem, Source: r.Source,
			Name: name, Paths: []string{path}, Safety: r.Safety, Why: r.Why, Restore: restore,
			ModTime: newestShallow(filepath.Join(path, "snapshots"), filepath.Join(path, "refs")),
		})
	}
	return out
}

type dockerDF struct {
	Type        string
	TotalCount  string
	Active      string
	Size        string
	Reclaimable string
}

// dockerItems asks the daemon what is reclaimable. Docker's data lives inside
// a VM disk image on macOS/Windows, so it can only be cleaned via the CLI.
func dockerItems(env *rules.Env) ([]*core.Item, string) {
	if _, err := env.LookPath("docker"); err != nil {
		return nil, ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "docker", "system", "df", "--format", "{{json .}}").Output()
	if err != nil {
		return nil, "docker: daemon not reachable, skipped"
	}
	var items []*core.Item
	sc := bufio.NewScanner(bytes.NewReader(out))
	for sc.Scan() {
		var row dockerDF
		if json.Unmarshal(sc.Bytes(), &row) != nil {
			continue
		}
		size := parseDockerSize(row.Reclaimable)
		if size <= 0 {
			continue
		}
		total, _ := strconv.Atoi(row.TotalCount)
		active, _ := strconv.Atoi(row.Active)
		var id, name string
		var cmd []string
		switch row.Type {
		case "Images":
			id, name, cmd = "docker.images", "unused images ("+strconv.Itoa(total-active)+")", []string{"docker", "image", "prune", "--all", "--force"}
		case "Containers":
			id, name, cmd = "docker.containers", "stopped containers ("+strconv.Itoa(total-active)+")", []string{"docker", "container", "prune", "--force"}
		case "Local Volumes":
			id, name, cmd = "docker.volumes", "unused volumes ("+strconv.Itoa(total-active)+")", []string{"docker", "volume", "prune", "--all", "--force"}
		case "Build Cache":
			id, name, cmd = "docker.build-cache", "build cache", []string{"docker", "builder", "prune", "--all", "--force"}
		default:
			continue
		}
		r := rules.ByID(id)
		it := &core.Item{
			ID: id, RuleID: id, Ecosystem: r.Ecosystem, Source: r.Source, Name: name,
			Size: size, SizeKnown: true, Safety: r.Safety, Why: r.Why, Command: cmd,
		}
		if env.GOOS != "linux" {
			it.Note = "Docker Desktop returns freed space to the host as its VM disk is trimmed"
		}
		items = append(items, it)
	}
	return items, ""
}

// parseDockerSize reads "1.234GB (35%)" — Docker uses decimal units.
func parseDockerSize(s string) int64 {
	s, _, _ = strings.Cut(strings.TrimSpace(s), " ")
	i := strings.IndexFunc(s, func(r rune) bool { return (r < '0' || r > '9') && r != '.' })
	if i <= 0 {
		return 0
	}
	n, err := strconv.ParseFloat(s[:i], 64)
	if err != nil {
		return 0
	}
	mult := map[string]float64{"B": 1, "kB": 1e3, "KB": 1e3, "MB": 1e6, "GB": 1e9, "TB": 1e12}[s[i:]]
	return int64(n * mult)
}
