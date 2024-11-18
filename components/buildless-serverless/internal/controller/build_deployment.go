package controller

import (
	"fmt"
	serverlessv1alpha2 "github.com/kyma-project/serverless/api/v1alpha2"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func buildDeployment(function *serverlessv1alpha2.Function) *appsv1.Deployment {
	fRuntime := function.Spec.Runtime

	labels := map[string]string{
		"app": function.Name,
	}

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-function-deployment", function.Name),
			Namespace: function.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: buildPodSpec(function, fRuntime),
			},
		},
	}
	return deployment
}

func buildPodSpec(function *serverlessv1alpha2.Function, fRuntime serverlessv1alpha2.Runtime) corev1.PodSpec {
	return corev1.PodSpec{
		Volumes: []corev1.Volume{
			{
				// used for writing sources (code&deps) to the sources dir
				Name: "sources",
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{},
				},
			},
			{
				// required by pip to save deps to .local dir
				Name: "local",
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{},
				},
			},
		},
		Containers: []corev1.Container{
			{
				Name:       fmt.Sprintf("%s-function-pod", function.Name),
				Image:      getRuntimeImage(fRuntime),
				WorkingDir: getWorkingSourcesDir(fRuntime),
				Command: []string{
					"sh",
					"-c",
					getRuntimeCommand(*function),
				},
				Env: getEnvs(fRuntime),
				VolumeMounts: []corev1.VolumeMount{
					{
						Name:      "sources",
						MountPath: getWorkingSourcesDir(fRuntime),
					},
					{
						Name:      "local",
						MountPath: "/.local",
					},
				},
				Ports: []corev1.ContainerPort{
					{
						ContainerPort: 80,
					},
				},
			},
		},
	}
}

func getRuntimeImage(runtime serverlessv1alpha2.Runtime) string {
	switch runtime {
	case serverlessv1alpha2.NodeJs20:
		return "europe-docker.pkg.dev/kyma-project/prod/function-runtime-nodejs20:main"
	case serverlessv1alpha2.Python312:
		return "europe-docker.pkg.dev/kyma-project/prod/function-runtime-python312:main"
	default:
		return ""
	}
}

func getWorkingSourcesDir(runtime serverlessv1alpha2.Runtime) string {
	switch runtime {
	case serverlessv1alpha2.NodeJs20:
		return "/usr/src/app/function"
	case serverlessv1alpha2.Python312:
		return "/kubeless"
	default:
		return ""
	}
}

func getRuntimeCommand(f serverlessv1alpha2.Function) string {
	runtime := f.Spec.Runtime
	switch runtime {
	case serverlessv1alpha2.NodeJs20:
		if /*f.Spec.Source.Inline.Dependencies != ""*/ true {
			// if deps are not empty use pip
			return `printf "${FUNC_HANDLER_SOURCE}" > handler.js;
printf "${FUNC_HANDLER_DEPENDENCIES}" > package.json;
npm install --prefer-offline --no-audit --progress=false;
cd ..;
npm start;`
		}
		return `printf "${FUNC_HANDLER_SOURCE}" > handler.js;
cd ..;
npm start;`
	case serverlessv1alpha2.Python312:
		if /*f.Spec.Source.Inline.Dependencies != ""*/ true {
			// if deps are not empty use npm
			return `printf "${FUNC_HANDLER_SOURCE}" > handler.py;
printf "${FUNC_HANDLER_DEPENDENCIES}" > requirements.txt;
pip install --user --no-cache-dir -r /kubeless/requirements.txt;
cd ..;
python /kubeless.py;`
		}
		return `printf "${FUNC_HANDLER_SOURCE}" > handler.py;
cd ..;
python /kubeless.py;`
	default:
		return ""
	}
}

func getEnvs(runtime serverlessv1alpha2.Runtime) []corev1.EnvVar {
	envs := []corev1.EnvVar{
		{
			Name:  "FUNC_HANDLER_SOURCE",
			Value: getFunctionSource(runtime),
		},
		{
			Name:  "FUNC_HANDLER_DEPENDENCIES",
			Value: getFunctionDependencies(runtime),
		},
	}
	if runtime == serverlessv1alpha2.Python312 {
		envs = append(envs, []corev1.EnvVar{
			{
				Name:  "MOD_NAME",
				Value: "handler",
			},
			{
				Name:  "FUNC_HANDLER",
				Value: "main",
			},
		}...)
	}
	return envs
}

func getFunctionSource(r serverlessv1alpha2.Runtime) string {
	switch r {
	case serverlessv1alpha2.NodeJs20:
		return `const _ = require('lodash')
module.exports = {
main: function(event, context) {
		return _.kebabCase('Hello World from Node.js 20 Function');
	}
}`
	case serverlessv1alpha2.Python312:
		return `import requests
def main(event, context):
	r = requests.get('https://swapi.dev/api/people/13')
	return r.json()
`
	default:
		return ""
	}
}

func getFunctionDependencies(r serverlessv1alpha2.Runtime) string {
	switch r {
	case serverlessv1alpha2.NodeJs20:
		return `{
  "name": "test-function-nodejs",
  "version": "1.0.0",
  "dependencies": {
	"lodash":"^4.17.20"
  }
}`
	case serverlessv1alpha2.Python312:
		return `requests==2.31.0
`
	default:
		return ""
	}
}
