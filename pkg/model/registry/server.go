// Copyright 2020-present Open Networking Foundation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package modelregistry

import (
	"context"
	"fmt"
	configmodelapi "github.com/onosproject/onos-config-model-go/api/onos/configmodel"
	"github.com/onosproject/onos-config-model-go/pkg/model"
	"github.com/onosproject/onos-config-model-go/pkg/model/plugin/compiler"
	"github.com/onosproject/onos-lib-go/pkg/northbound"
	"google.golang.org/grpc"
	"os"
	"path/filepath"
)

func NewService(registry *ConfigModelRegistry, compiler *plugincompiler.PluginCompiler) northbound.Service {
	return &Service{
		registry: registry,
		compiler: compiler,
	}
}

type Service struct {
	registry *ConfigModelRegistry
	compiler *plugincompiler.PluginCompiler
}

func (s *Service) Register(r *grpc.Server) {
	server := &Server{
		registry: s.registry,
		compiler: s.compiler,
	}
	configmodelapi.RegisterConfigModelRegistryServiceServer(r, server)
}

var _ northbound.Service = &Service{}

// Server is a registry server
type Server struct {
	registry *ConfigModelRegistry
	compiler *plugincompiler.PluginCompiler
}

func (s *Server) GetModel(ctx context.Context, request *configmodelapi.GetModelRequest) (*configmodelapi.GetModelResponse, error) {
	log.Debugf("Received GetModelRequest %+v", request)
	modelInfo, err := s.registry.GetModel(configmodel.Name(request.Name), configmodel.Version(request.Version))
	if err != nil {
		log.Warnf("GetModelRequest %+v failed: %v", request, err)
		return nil, err
	}

	var modules []*configmodelapi.ConfigModule
	for _, moduleInfo := range modelInfo.Modules {
		modules = append(modules, &configmodelapi.ConfigModule{
			Name:         string(moduleInfo.Name),
			Organization: string(moduleInfo.Organization),
			Version:      string(moduleInfo.Version),
			Data:         moduleInfo.Data,
		})
	}
	response := &configmodelapi.GetModelResponse{
		Model: &configmodelapi.ConfigModel{
			Name:    string(modelInfo.Name),
			Version: string(modelInfo.Version),
			Modules: modules,
		},
	}
	log.Debugf("Sending GetModelResponse %+v", response)
	return response, nil
}

func (s *Server) ListModels(ctx context.Context, request *configmodelapi.ListModelsRequest) (*configmodelapi.ListModelsResponse, error) {
	log.Debugf("Received ListModelsRequest %+v", request)
	modelInfos, err := s.registry.ListModels()
	if err != nil {
		log.Warnf("ListModelsRequest %+v failed: %v", request, err)
		return nil, err
	}

	var models []*configmodelapi.ConfigModel
	for _, modelInfo := range modelInfos {
		var modules []*configmodelapi.ConfigModule
		for _, module := range modelInfo.Modules {
			modules = append(modules, &configmodelapi.ConfigModule{
				Name:         string(module.Name),
				Organization: string(module.Organization),
				Version:      string(module.Version),
				Data:         module.Data,
			})
		}
		models = append(models, &configmodelapi.ConfigModel{
			Name:    string(modelInfo.Name),
			Version: string(modelInfo.Version),
			Modules: modules,
		})
	}
	response := &configmodelapi.ListModelsResponse{
		Models: models,
	}
	log.Debugf("Sending ListModelsResponse %+v", response)
	return response, nil
}

func (s *Server) PushModel(ctx context.Context, request *configmodelapi.PushModelRequest) (*configmodelapi.PushModelResponse, error) {
	log.Debugf("Received PushModelRequest %+v", request)
	var moduleInfos []configmodel.ModuleInfo
	for _, module := range request.Model.Modules {
		moduleInfos = append(moduleInfos, configmodel.ModuleInfo{
			Name:         configmodel.Name(module.Name),
			Organization: module.Organization,
			Version:      configmodel.Version(module.Version),
			Data:         module.Data,
		})
	}
	modelInfo := configmodel.ModelInfo{
		Name:    configmodel.Name(request.Model.Name),
		Version: configmodel.Version(request.Model.Version),
		Modules: moduleInfos,
		Plugin: configmodel.PluginInfo{
			Name:    configmodel.Name(request.Model.Name),
			Version: configmodel.Version(request.Model.Version),
			Target:  request.Model.Target,
			Replace: request.Model.Replace,
			File:    getPluginFile(request.Model.Name, request.Model.Version),
		},
	}
	err := s.compiler.CompilePlugin(modelInfo)
	if err != nil {
		log.Warnf("PushModelRequest %+v failed: %v", request, err)
		return nil, err
	}
	err = s.registry.AddModel(modelInfo)
	response := &configmodelapi.PushModelResponse{}
	log.Debugf("Sending PushModelResponse %+v", response)
	return response, nil
}

func (s *Server) DeleteModel(ctx context.Context, request *configmodelapi.DeleteModelRequest) (*configmodelapi.DeleteModelResponse, error) {
	log.Debugf("Received DeleteModelRequest %+v", request)
	err := s.registry.RemoveModel(configmodel.Name(request.Name), configmodel.Version(request.Version))
	if err != nil {
		log.Warnf("DeleteModelRequest %+v failed: %v", request, err)
		return nil, err
	}
	path := filepath.Join(s.registry.Config.Path, getPluginFile(request.Name, request.Version))
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		os.Remove(path)
	}
	response := &configmodelapi.DeleteModelResponse{}
	log.Debugf("Sending DeleteModelResponse %+v", response)
	return response, nil
}

func getPluginFile(name, version string) string {
	return fmt.Sprintf("%s-%s.so", name, version)
}

var _ configmodelapi.ConfigModelRegistryServiceServer = &Server{}