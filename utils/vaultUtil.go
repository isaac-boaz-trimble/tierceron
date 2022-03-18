package utils

import (
	"log"
	"strings"
	"tierceron/vaulthelper/kv"
	helperkv "tierceron/vaulthelper/kv"
	sys "tierceron/vaulthelper/system"
)

// Helper to easiliy intialize a vault and a mod all at once.
func InitVaultMod(config *DriverConfig) (*DriverConfig, *helperkv.Modifier, *sys.Vault, error) {
	vault, err := sys.NewVault(config.Insecure, config.VaultAddress, config.Env, false, false, config.ExitOnFailure, config.Log)
	if err != nil {
		LogErrorObject(config, err, false)
		return config, nil, nil, err
	}
	vault.SetToken(config.Token)

	mod, err := helperkv.NewModifier(config.Insecure, config.Token, config.VaultAddress, config.Env, config.Regions, config.Log)
	if err != nil {
		LogErrorObject(config, err, false)
		return config, nil, nil, err
	}
	mod.Env = config.Env
	mod.Version = "0"
	mod.VersionFilter = config.VersionFilter

	return config, mod, vault, nil
}

func GetAcceptedTemplatePaths(config *DriverConfig, modCheck *kv.Modifier, templatePaths []string) ([]string, error) {
	var acceptedTemplatePaths []string
	serviceMap := make(map[string]bool)

	if strings.Contains(config.EnvRaw, "_") {
		config.EnvRaw = strings.Split(config.EnvRaw, "_")[0]
	}

	if modCheck != nil {
		envVersion := SplitEnv(config.Env)
		serviceInterface, err := modCheck.ListEnv("super-secrets/" + envVersion[0])
		modCheck.Env = config.Env
		if err != nil {
			return nil, err
		}
		if serviceInterface == nil || serviceInterface.Data["keys"] == nil {
			return templatePaths, nil
		}

		serviceList := serviceInterface.Data["keys"]
		for _, data := range serviceList.([]interface{}) {
			serviceMap[data.(string)] = true
		}

		for _, templatePath := range templatePaths {
			if len(config.ProjectSections) > 0 { //Filter by project
				for _, projectSection := range config.ProjectSections {
					if strings.Contains(templatePath, "/"+projectSection+"/") {
						listValues, err := modCheck.ListEnv("super-secrets/" + strings.Split(config.EnvRaw, ".")[0] + config.SectionKey + config.ProjectSections[0] + "/" + config.SectionName)
						if err != nil || listValues == nil {
							LogErrorObject(config, err, false)
							LogInfo(config, "Couldn't list services for project path")
							continue
						}
						for _, valuesPath := range listValues.Data {
							for _, service := range valuesPath.([]interface{}) {
								serviceMap[service.(string)] = true
							}
						}
					}
				}
			}
		}
	}
	for _, templatePath := range templatePaths {
		templatePathParts := strings.Split(templatePath, "/")
		service := templatePathParts[len(templatePathParts)-2]

		if _, ok := serviceMap[service]; ok {
			if config.SectionKey == "" || config.SectionKey == "/" {
				acceptedTemplatePaths = append(acceptedTemplatePaths, templatePath)
			} else {
				for _, sectionProject := range config.ProjectSections {
					if strings.Contains(templatePath, sectionProject) {
						acceptedTemplatePaths = append(acceptedTemplatePaths, templatePath)
					}
				}
			}
		} else {
			if config.SectionKey != "" && config.SectionKey != "/" {
				for _, sectionProject := range config.ProjectSections {
					if strings.Contains(templatePath, "/"+sectionProject+"/") {
						acceptedTemplatePaths = append(acceptedTemplatePaths, templatePath)
					}
				}
			}
		}
	}

	if len(acceptedTemplatePaths) > 0 {
		templatePaths = acceptedTemplatePaths
	}

	return templatePaths, nil
}

// Helper to easiliy intialize a vault and a mod all at once.
func InitVaultModForPlugin(pluginConfig map[string]interface{}, logger *log.Logger) (*DriverConfig, *helperkv.Modifier, *sys.Vault, error) {
	exitOnFailure := false
	if _, ok := pluginConfig["exitOnFailure"]; ok {
		exitOnFailure = pluginConfig["exitOnFailure"].(bool)
	}
	config := DriverConfig{
		Insecure:       pluginConfig["insecure"].(bool),
		Token:          pluginConfig["token"].(string),
		VaultAddress:   pluginConfig["address"].(string),
		Env:            pluginConfig["env"].(string),
		Regions:        pluginConfig["regions"].([]string),
		SecretMode:     true, //  "Only override secret values in templates?"
		ServicesWanted: []string{},
		StartDir:       append([]string{}, ""),
		EndDir:         "",
		WantCerts:      false,
		GenAuth:        false,
		ExitOnFailure:  exitOnFailure,
		Log:            logger,
	}

	return InitVaultMod(&config)
}