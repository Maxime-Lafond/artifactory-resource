package commands

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jfrogdev/jfrog-cli-go/artifactory/utils"
	"github.com/jfrogdev/jfrog-cli-go/utils/cliutils"
	"github.com/jfrogdev/jfrog-cli-go/utils/cliutils/log"
	"path/filepath"
	"sort"
	"strings"
)

func BuildPublish(buildName, buildNumber string, flags *utils.BuildInfoFlags) error {
	err := utils.PreCommandSetup(flags)
	if err != nil {
		return err
	}

	buildData, err := utils.ReadBuildInfoFiles(buildName, buildNumber)
	if err != nil {
		return err
	}

	if len(buildData) == 0 {
		return cliutils.CheckError(fmt.Errorf("Can't find any files related to build name: %q, number: %q", buildName, buildNumber))
	}
	sort.Sort(buildData)
	buildInfo := createNewBuildInfo()
	buildInfo.Name = buildName
	buildInfo.Number = buildNumber
	buildGeneralDetails, err := utils.ReadBuildInfoGeneralDetails(buildName, buildNumber)
	if err != nil {
		return err
	}
	buildInfo.Started = buildGeneralDetails.Timestamp.Format("2006-01-02T15:04:05.000-0700")
	artifactsSet, dependenciesSet, env, err := prepareBuildInfoData(buildData, createIncludeFilter(flags), createExcludeFilter(flags))
	if err != nil {
		return err
	}
	if len(env) != 0 {
		buildInfo.Propertires = env
	}
	module := createModule(buildName, artifactsSet, dependenciesSet)
	buildInfo.Modules = append(buildInfo.Modules, module)
	marshaledBuildInfo, err := json.Marshal(buildInfo)
	if cliutils.CheckError(err) != nil {
		return err
	}
	if flags.IsDryRun() {
		fmt.Println(cliutils.IndentJson(marshaledBuildInfo))
		return nil
	}
	httpClientsDetails := utils.GetArtifactoryHttpClientDetails(flags.ArtDetails)
	utils.SetContentType("application/vnd.org.jfrog.artifactory+json", &httpClientsDetails.Headers)
	log.Info("Deploying build info...")
	resp, body, err := utils.PublishBuildInfo(flags.ArtDetails.Url, marshaledBuildInfo, httpClientsDetails)
	if err != nil {
		return err
	}
	if resp.StatusCode != 204 {
		return cliutils.CheckError(errors.New("Artifactory response: " + resp.Status + "\n" + cliutils.IndentJson(body)))
	}

	log.Debug("Artifactory response:", resp.Status)
	log.Info("Build info successfully deployed. Browse it in Artifactory under " + flags.ArtDetails.Url + "webapp/builds/" + buildName + "/" + buildNumber)
	if err = utils.RemoveBuildDir(buildName, buildNumber); err != nil {
		return err
	}
	return nil
}

func prepareBuildInfoData(artifactsDataWrapper utils.BuildInfo, includeFilter, excludeFilter filterFunc) ([]utils.ArtifactsBuildInfo, []utils.DependenciesBuildInfo, utils.BuildEnv, error) {
	var artifacts []utils.ArtifactsBuildInfo
	var dependencies []utils.DependenciesBuildInfo
	var env utils.BuildEnv
	env = make(map[string]string)
	for _, buildInfoData := range artifactsDataWrapper {
		switch {
		case buildInfoData.Artifacts != nil:
			for _, v := range buildInfoData.Artifacts {
				artifacts = append(artifacts, v)
			}
		case buildInfoData.Dependencies != nil:
			for _, v := range buildInfoData.Dependencies {
				dependencies = append(dependencies, v)
			}
		case buildInfoData.Env != nil:
			envAfterIncludeFilter, e := includeFilter(buildInfoData.Env)
			if cliutils.CheckError(e) != nil {
				return artifacts, dependencies, env, e
			}
			envAfterExcludeFilter, e := excludeFilter(envAfterIncludeFilter)
			if cliutils.CheckError(e) != nil {
				return artifacts, dependencies, env, e
			}
			for k, v := range envAfterExcludeFilter {
				env[k] = v
			}
		}
	}
	return artifacts, dependencies, env, nil
}

func createModule(buildName string, artifacts []utils.ArtifactsBuildInfo, dependencies []utils.DependenciesBuildInfo) (module *Modules) {
	module = createDefaultModule(buildName)
	if artifacts != nil && len(artifacts) > 0 {
		module.Artifacts = append(module.Artifacts, artifacts...)
	}
	if dependencies != nil && len(dependencies) > 0 {
		module.Dependencies = append(module.Dependencies, dependencies...)
	}
	return
}

type BuildInfo struct {
	Name        string         `json:"name,omitempty"`
	Number      string         `json:"number,omitempty"`
	Agent       *CliAgent      `json:"agent,omitempty"`
	BuildAgent  *CliAgent      `json:"buildAgent,omitempty"`
	Modules     []*Modules     `json:"modules,omitempty"`
	Started     string         `json:"started,omitempty"`
	Propertires utils.BuildEnv `json:"properties,omitempty"`
}

type CliAgent struct {
	Name    string `json:"name,omitempty"`
	Version string `json:"version,omitempty"`
}

type Modules struct {
	Properties   map[string][]string           `json:"properties,omitempty"`
	Id           string                        `json:"id,omitempty"`
	Artifacts    []utils.ArtifactsBuildInfo    `json:"artifacts,omitempty"`
	Dependencies []utils.DependenciesBuildInfo `json:"dependencies,omitempty"`
}

func createNewBuildInfo() (buildInfo *BuildInfo) {
	buildInfo = new(BuildInfo)
	buildInfo.Agent = new(CliAgent)
	buildInfo.Agent.Name = cliutils.CliAgent
	buildInfo.Agent.Version = cliutils.GetVersion()
	buildInfo.BuildAgent = new(CliAgent)
	buildInfo.BuildAgent.Name = "GENERIC"
	buildInfo.BuildAgent.Version = cliutils.GetVersion()
	buildInfo.Modules = make([]*Modules, 0)
	return
}

func createDefaultModule(buildName string) (module *Modules) {
	module = new(Modules)
	module.Id = buildName
	module.Properties = make(map[string][]string)
	module.Artifacts = make([]utils.ArtifactsBuildInfo, 0)
	module.Dependencies = make([]utils.DependenciesBuildInfo, 0)
	return
}

type filterFunc func(map[string]string) (map[string]string, error)

func createIncludeFilter(flags *utils.BuildInfoFlags) filterFunc {
	includePatterns := strings.Split(flags.EnvInclude, ";")
	return func(tempMap map[string]string) (map[string]string, error) {
		result := make(map[string]string)
		for k, v := range tempMap {
			for _, filterPattern := range includePatterns {
				bool, err := filepath.Match(filterPattern, k)
				if cliutils.CheckError(err) != nil {
					return nil, err
				}
				if bool == true {
					result[k] = v
					break
				}
			}
		}
		return result, nil
	}
}

func createExcludeFilter(flags *utils.BuildInfoFlags) filterFunc {
	excludePattern := strings.Split(flags.EnvExclude, ";")
	return func(tempMap map[string]string) (map[string]string, error) {
		result := make(map[string]string)
		for k, v := range tempMap {
			include := true
			for _, filterPattern := range excludePattern {
				bool, err := filepath.Match(filterPattern, k)
				if cliutils.CheckError(err) != nil {
					return nil, err
				}
				if bool == true {
					include = false
					break
				}
			}
			if include {
				result[k] = v
			}
		}
		return result, nil
	}
}
