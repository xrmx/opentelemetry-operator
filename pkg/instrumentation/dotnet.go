// Copyright The OpenTelemetry Authors
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

package instrumentation

import (
	"fmt"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"

	"github.com/open-telemetry/opentelemetry-operator/apis/v1alpha1"
)

const (
	envDotNetCoreClrEnableProfiling     = "CORECLR_ENABLE_PROFILING"
	envDotNetCoreClrProfiler            = "CORECLR_PROFILER"
	envDotNetCoreClrProfilerPath        = "CORECLR_PROFILER_PATH"
	envDotNetAdditionalDeps             = "DOTNET_ADDITIONAL_DEPS"
	envDotNetSharedStore                = "DOTNET_SHARED_STORE"
	envDotNetStartupHook                = "DOTNET_STARTUP_HOOKS"
	envDotNetOTelAutoHome               = "OTEL_DOTNET_AUTO_HOME"
	dotNetCoreClrEnableProfilingEnabled = "1"
	dotNetCoreClrProfilerId             = "{918728DD-259F-4A6A-AC2B-B85E1B658318}"
	dotNetCoreClrProfilerPath           = "/otel-auto-instrumentation/OpenTelemetry.AutoInstrumentation.Native.so"
	dotNetAdditionalDepsPath            = "/otel-auto-instrumentation/AdditionalDeps"
	dotNetOTelAutoHomePath              = "/otel-auto-instrumentation"
	dotNetSharedStorePath               = "/otel-auto-instrumentation/store"
	dotNetStartupHookPath               = "/otel-auto-instrumentation/netcoreapp3.1/OpenTelemetry.AutoInstrumentation.StartupHook.dll"
)

func injectDotNetSDK(logger logr.Logger, dotNetSpec v1alpha1.DotNet, pod corev1.Pod, index int) corev1.Pod {
	// caller checks if there is at least one container
	container := pod.Spec.Containers[index]

	// inject env vars
	for _, env := range dotNetSpec.Env {
		idx := getIndexOfEnv(container.Env, env.Name)
		if idx == -1 {
			container.Env = append(container.Env, env)
		}
	}

	const (
		doNotConcatEnvValues = false
		concatEnvValues      = true
	)

	if !trySetEnvVar(logger, &container, envDotNetCoreClrEnableProfiling, dotNetCoreClrEnableProfilingEnabled, doNotConcatEnvValues) {
		return pod
	}

	if !trySetEnvVar(logger, &container, envDotNetCoreClrProfiler, dotNetCoreClrProfilerId, doNotConcatEnvValues) {
		return pod
	}

	if !trySetEnvVar(logger, &container, envDotNetCoreClrProfilerPath, dotNetCoreClrProfilerPath, doNotConcatEnvValues) {
		return pod
	}

	if !trySetEnvVar(logger, &container, envDotNetStartupHook, dotNetStartupHookPath, concatEnvValues) {
		return pod
	}

	if !trySetEnvVar(logger, &container, envDotNetAdditionalDeps, dotNetAdditionalDepsPath, concatEnvValues) {
		return pod
	}

	if !trySetEnvVar(logger, &container, envDotNetOTelAutoHome, dotNetOTelAutoHomePath, doNotConcatEnvValues) {
		return pod
	}

	if !trySetEnvVar(logger, &container, envDotNetSharedStore, dotNetSharedStorePath, concatEnvValues) {
		return pod
	}

	container.VolumeMounts = append(container.VolumeMounts, corev1.VolumeMount{
		Name:      volumeName,
		MountPath: "/otel-auto-instrumentation",
	})

	// We just inject Volumes and init containers for the first processed container
	if isInitContainerMissing(pod) {
		pod.Spec.Volumes = append(pod.Spec.Volumes, corev1.Volume{
			Name: volumeName,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			}})

		pod.Spec.InitContainers = append(pod.Spec.InitContainers, corev1.Container{
			Name:    initContainerName,
			Image:   dotNetSpec.Image,
			Command: []string{"cp", "-a", "/autoinstrumentation/.", "/otel-auto-instrumentation/"},
			VolumeMounts: []corev1.VolumeMount{{
				Name:      volumeName,
				MountPath: "/otel-auto-instrumentation",
			}},
		})
	}

	pod.Spec.Containers[index] = container
	return pod
}

func trySetEnvVar(logger logr.Logger, container *corev1.Container, envVarName string, envVarValue string, concatValues bool) bool {
	idx := getIndexOfEnv(container.Env, envVarName)
	if idx < 0 {
		container.Env = append(container.Env, corev1.EnvVar{
			Name:  envVarName,
			Value: envVarValue,
		})
		return true
	}

	if container.Env[idx].ValueFrom != nil {
		// TODO add to status object or submit it as an event
		logger.Info("Skipping DotNet SDK injection, the container defines env var value via ValueFrom", "envVar", envVarName, "container", container.Name)
		return false
	}

	if concatValues {
		container.Env[idx].Value = fmt.Sprintf("%s:%s", container.Env[idx].Value, envVarValue)
	}

	return true
}