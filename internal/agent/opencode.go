package agent

import (
	"github.com/arcaku-labs/pindah/internal/archive"
)

// OpenCodeHomes are the directories backup opencode reads.
func OpenCodeHomes(configHome, skillsHome, shareHome string) map[string]string {
	return map[string]string{
		"opencode-config": configHome,
		"opencode-skills": skillsHome,
		"opencode-share":  shareHome,
	}
}

// CollectOpenCode packs the three opencode homes. Missing paths are omitted.
func CollectOpenCode(configHome, skillsHome, shareHome string) ([]archive.Source, error) {
	named, err := files(configHome,
		"AGENTS.md",
		"opencode.jsonc",
		"ocx.jsonc",
		"config.json",
		"package.json",
		"package-lock.json",
	)
	if err != nil {
		return nil, err
	}
	ts, err := glob(configHome, "plugins", "*.ts")
	if err != nil {
		return nil, err
	}
	js, err := glob(configHome, "plugins", "*.js")
	if err != nil {
		return nil, err
	}
	profiles, err := tree(configHome, "profiles")
	if err != nil {
		return nil, err
	}
	configSkills, err := tree(configHome, "skills")
	if err != nil {
		return nil, err
	}
	extraSkills, err := tree(skillsHome, "skills")
	if err != nil {
		return nil, err
	}
	auth, err := files(shareHome, "auth.json")
	if err != nil {
		return nil, err
	}

	var out []archive.Source
	out = append(out, withHome("opencode-config", append(named, ts...))...)
	out = append(out, withHome("opencode-config", js)...)
	out = append(out, withHome("opencode-config", profiles)...)
	out = append(out, withHome("opencode-config", configSkills)...)
	out = append(out, withHome("opencode-skills", extraSkills)...)
	out = append(out, withHome("opencode-share", auth)...)
	return out, nil
}
