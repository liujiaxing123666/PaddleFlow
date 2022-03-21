/*
Copyright (c) 2022 PaddlePaddle Authors. All Rights Reserve.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package executor

import (
	"testing"

	pdv1 "github.com/paddleflow/paddle-operator/api/v1"
	"github.com/stretchr/testify/assert"

	"paddleflow/pkg/common/schema"
	"paddleflow/pkg/job/api"
)

func TestPatchPaddleJobVariable(t *testing.T) {
	confEnv := make(map[string]string)
	initConfigsForTest(confEnv)
	confEnv[schema.EnvJobType] = string(schema.TypePaddleJob)
	confEnv[schema.EnvJobFlavour] = "gpu"
	// init for paddle's ps mode
	confEnv[schema.EnvJobPServerCommand] = "sleep 30"
	confEnv[schema.EnvJobWorkerCommand] = "sleep 30"
	confEnv[schema.EnvJobPServerReplicas] = "2"
	confEnv[schema.EnvJobWorkerReplicas] = "2"
	confEnv[schema.EnvJobPServerFlavour] = "cpu"
	confEnv[schema.EnvJobWorkerFlavour] = "gpu"
	confEnv[schema.EnvJobFsID] = "fs1"

	pfjob := &api.PFJob{
		Conf: schema.Conf{
			Name:    "confName",
			Env:     confEnv,
			Command: "sleep 3600",
			Image:   "nginx",
		},
		JobType: schema.TypePaddleJob,
	}

	tests := []struct {
		caseName      string
		jobMode       string
		additionalEnv map[string]string
		actualValue   *pdv1.PaddleJob
		expectValue   string
		errMsg        string
	}{
		{
			caseName:    "psMode",
			jobMode:     schema.EnvJobModePS,
			actualValue: &pdv1.PaddleJob{},
			expectValue: "paddle",
		},
		{
			caseName:    "collectiveMode",
			jobMode:     schema.EnvJobModeCollective,
			actualValue: &pdv1.PaddleJob{},
			expectValue: "paddle",
		},
		{
			caseName: "fromUserPath",
			jobMode:  schema.EnvJobModePS,
			additionalEnv: map[string]string{
				schema.EnvJobNamespace: "N2",
			},
			actualValue: &pdv1.PaddleJob{},
			expectValue: "paddle",
		},
	}

	for _, test := range tests {
		if len(test.additionalEnv) != 0 {
			for k, v := range test.additionalEnv {
				pfjob.Conf.SetEnv(k, v)
			}
		}
		pfjob.Conf.SetEnv(schema.EnvJobMode, test.caseName)
		pfjob.JobMode = test.jobMode
		// yaml content
		extRuntimeConf, err := pfjob.GetExtRuntimeConf(pfjob.Conf.GetFS(), pfjob.Conf.GetYamlPath())
		if err != nil {
			t.Errorf(err.Error())
		}
		kubeJob := KubeJob{
			ID:                  "randomID",
			Name:                "randomName",
			Namespace:           "namespace",
			JobType:             schema.TypePaddleJob,
			JobMode:             pfjob.JobMode,
			Image:               pfjob.Conf.GetImage(),
			Command:             pfjob.Conf.GetCommand(),
			Env:                 pfjob.Conf.GetEnv(),
			VolumeName:          pfjob.Conf.GetFS(),
			PVCName:             "PVCName",
			Priority:            pfjob.Conf.GetPriority(),
			QueueName:           pfjob.Conf.GetQueueName(),
			YamlTemplateContent: extRuntimeConf,
		}
		jobModeParams := JobModeParams{
			JobFlavour:      pfjob.Conf.GetFlavour(),
			PServerReplicas: pfjob.Conf.GetPSReplicas(),
			PServerFlavour:  pfjob.Conf.GetFlavour(),
			PServerCommand:  pfjob.Conf.GetPSCommand(),
			WorkerReplicas:  pfjob.Conf.GetWorkerReplicas(),
			WorkerFlavour:   pfjob.Conf.GetWorkerFlavour(),
			WorkerCommand:   pfjob.Conf.GetWorkerCommand(),
		}
		paddleJob := PaddleJob{
			KubeJob:       kubeJob,
			JobModeParams: jobModeParams,
		}
		pdj := test.actualValue
		if err := paddleJob.createJobFromYaml(pdj); err != nil {
			t.Errorf("create job failed, err %v", err)
		}
		err = paddleJob.patchPaddleJobVariable(pdj, test.caseName)
		if err != nil {
			t.Errorf(err.Error())
		}

		t.Logf("case[%s]\n pdj=%+v \n pdj.Spec.SchedulingPolicy=%+v \n pdj.Spec.Worker=%+v", test.caseName, pdj, pdj.Spec.SchedulingPolicy, pdj.Spec.Worker)

		assert.NotEmpty(t, pdj.Spec.Worker)
		assert.NotEmpty(t, pdj.Spec.Worker.Template.Spec.Containers)
		assert.NotEmpty(t, pdj.Spec.Worker.Template.Spec.Containers[0].Name)
		if test.jobMode == schema.EnvJobModePS {
			assert.NotEmpty(t, pdj.Spec.PS)
			assert.NotEmpty(t, pdj.Spec.PS.Template.Spec.Containers)
			assert.NotEmpty(t, pdj.Spec.PS.Template.Spec.Containers[0].Name)
		}
		assert.NotEmpty(t, pdj.Spec.SchedulingPolicy)
		t.Logf("len(pdj.Spec.SchedulingPolicy.MinResources)=%d", len(pdj.Spec.SchedulingPolicy.MinResources))
		assert.NotZero(t, len(pdj.Spec.SchedulingPolicy.MinResources))
		t.Logf("case[%s] SchedulingPolicy=%+v MinAvailable=%+v MinResources=%+v ", test.caseName, pdj.Spec.SchedulingPolicy,
			*pdj.Spec.SchedulingPolicy.MinAvailable, pdj.Spec.SchedulingPolicy.MinResources)
		assert.NotEmpty(t, pdj.Spec.Worker.Template.Spec.Containers[0].VolumeMounts)
	}

}