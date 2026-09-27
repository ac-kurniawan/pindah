package agent

import (
	"github.com/arcaku-labs/pindah/internal/archive"
)

// OMPHomes are the directories backup omp reads. Key is the archive home-id.
func OMPHomes(home string) map[string]string {
	return map[string]string{"omp": home}
}

// CollectOMP returns allowlisted files under the omp home. Missing paths are omitted.
func CollectOMP(home string) ([]archive.Source, error) {
	named, err := files(home,
		"AGENTS.md",
		"mcp.json",
		"marketplaces.json",
		"models.yaml",
		"agent/AGENTS.md",
		"agent/config.yml",
		"agent/mcp.json",
		"plugins/installed_plugins.json",
		"plugins/omp-plugins.lock.json",
		"plugins/package.json",
		"plugins/bun.lock",
	)
	if err != nil {
		return nil, err
	}
	ext, err := glob(home, "agent/extensions", "*.ts")
	if err != nil {
		return nil, err
	}
	skills, err := tree(home, "skills")
	if err != nil {
		return nil, err
	}
	out := append(named, ext...)
	out = append(out, skills...)
	return withHome("omp", out), nil
}
