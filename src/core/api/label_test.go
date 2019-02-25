// Copyright 2018 Project Harbor Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/goharbor/harbor/src/common"
	"github.com/goharbor/harbor/src/common/dao"
	"github.com/goharbor/harbor/src/common/models"
	"github.com/goharbor/harbor/src/replication"
	rep_models "github.com/goharbor/harbor/src/replication/models"
	rep_dao "github.com/goharbor/harbor/src/replication/ng/dao"
	dao_models "github.com/goharbor/harbor/src/replication/ng/dao/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	labelAPIBasePath = "/api/labels"
	labelID          int64
)

func TestLabelAPIPost(t *testing.T) {
	postFunc := func(resp *httptest.ResponseRecorder) error {
		id, err := parseResourceID(resp)
		if err != nil {
			return err
		}
		labelID = id
		return nil
	}

	cases := []*codeCheckingCase{
		// 401
		{
			request: &testingRequest{
				method: http.MethodPost,
				url:    labelAPIBasePath,
			},
			code: http.StatusUnauthorized,
		},

		// 400
		{
			request: &testingRequest{
				method:     http.MethodPost,
				url:        labelAPIBasePath,
				bodyJSON:   &models.Label{},
				credential: nonSysAdmin,
			},
			code: http.StatusBadRequest,
		},

		// 403 non-sysadmin try to create global label
		{
			request: &testingRequest{
				method: http.MethodPost,
				url:    labelAPIBasePath,
				bodyJSON: &models.Label{
					Name:  "test",
					Scope: common.LabelScopeGlobal,
				},
				credential: nonSysAdmin,
			},
			code: http.StatusForbidden,
		},

		// 403 non-member user try to create project label
		{
			request: &testingRequest{
				method: http.MethodPost,
				url:    labelAPIBasePath,
				bodyJSON: &models.Label{
					Name:      "test",
					Scope:     common.LabelScopeProject,
					ProjectID: 1,
				},
				credential: nonSysAdmin,
			},
			code: http.StatusForbidden,
		},

		// 403 developer try to create project label
		{
			request: &testingRequest{
				method: http.MethodPost,
				url:    labelAPIBasePath,
				bodyJSON: &models.Label{
					Name:      "test",
					Scope:     common.LabelScopeProject,
					ProjectID: 1,
				},
				credential: projDeveloper,
			},
			code: http.StatusForbidden,
		},

		// 404 non-exist project
		{
			request: &testingRequest{
				method: http.MethodPost,
				url:    labelAPIBasePath,
				bodyJSON: &models.Label{
					Name:      "test",
					Scope:     common.LabelScopeProject,
					ProjectID: 10000,
				},
				credential: projAdmin,
			},
			code: http.StatusNotFound,
		},

		// 200
		{
			request: &testingRequest{
				method: http.MethodPost,
				url:    labelAPIBasePath,
				bodyJSON: &models.Label{
					Name:      "test",
					Scope:     common.LabelScopeProject,
					ProjectID: 1,
				},
				credential: projAdmin,
			},
			code:     http.StatusCreated,
			postFunc: postFunc,
		},

		// 409
		{
			request: &testingRequest{
				method: http.MethodPost,
				url:    labelAPIBasePath,
				bodyJSON: &models.Label{
					Name:      "test",
					Scope:     common.LabelScopeProject,
					ProjectID: 1,
				},
				credential: projAdmin,
			},
			code: http.StatusConflict,
		},
	}

	runCodeCheckingCases(t, cases...)
}

func TestLabelAPIGet(t *testing.T) {
	cases := []*codeCheckingCase{
		// 400
		{
			request: &testingRequest{
				method: http.MethodGet,
				url:    fmt.Sprintf("%s/%d", labelAPIBasePath, 0),
			},
			code: http.StatusBadRequest,
		},

		// 404
		{
			request: &testingRequest{
				method: http.MethodGet,
				url:    fmt.Sprintf("%s/%d", labelAPIBasePath, 1000),
			},
			code: http.StatusNotFound,
		},

		// 200
		{
			request: &testingRequest{
				method: http.MethodGet,
				url:    fmt.Sprintf("%s/%d", labelAPIBasePath, labelID),
			},
			code: http.StatusOK,
		},
	}
	runCodeCheckingCases(t, cases...)
}

func TestLabelAPIList(t *testing.T) {
	cases := []*codeCheckingCase{
		// 400 no scope query string
		{
			request: &testingRequest{
				method: http.MethodGet,
				url:    labelAPIBasePath,
			},
			code: http.StatusBadRequest,
		},

		// 400 invalid scope
		{
			request: &testingRequest{
				method: http.MethodGet,
				url:    labelAPIBasePath,
				queryStruct: struct {
					Scope string `url:"scope"`
				}{
					Scope: "invalid_scope",
				},
			},
			code: http.StatusBadRequest,
		},

		// 400 invalid project_id
		{
			request: &testingRequest{
				method: http.MethodGet,
				url:    labelAPIBasePath,
				queryStruct: struct {
					Scope     string `url:"scope"`
					ProjectID int64  `url:"project_id"`
				}{
					Scope:     "p",
					ProjectID: 0,
				},
			},
			code: http.StatusBadRequest,
		},
	}
	runCodeCheckingCases(t, cases...)

	// 200
	labels := []*models.Label{}
	err := handleAndParse(&testingRequest{
		method: http.MethodGet,
		url:    labelAPIBasePath,
		queryStruct: struct {
			Scope     string `url:"scope"`
			ProjectID int64  `url:"project_id"`
			Name      string `url:"name"`
		}{
			Scope:     "p",
			ProjectID: 1,
			Name:      "tes",
		},
	}, &labels)
	require.Nil(t, err)
	assert.Equal(t, 1, len(labels))

	err = handleAndParse(&testingRequest{
		method: http.MethodGet,
		url:    labelAPIBasePath,
		queryStruct: struct {
			Scope     string `url:"scope"`
			ProjectID int64  `url:"project_id"`
			Name      string `url:"name"`
		}{
			Scope:     "p",
			ProjectID: 1,
			Name:      "dev",
		},
	}, &labels)
	require.Nil(t, err)
	assert.Equal(t, 0, len(labels))
}

func TestLabelAPIPut(t *testing.T) {
	cases := []*codeCheckingCase{
		// 401
		{
			request: &testingRequest{
				method: http.MethodPut,
				url:    fmt.Sprintf("%s/%d", labelAPIBasePath, labelID),
			},
			code: http.StatusUnauthorized,
		},

		// 400
		{
			request: &testingRequest{
				method:     http.MethodPut,
				url:        fmt.Sprintf("%s/%d", labelAPIBasePath, 0),
				credential: nonSysAdmin,
			},
			code: http.StatusBadRequest,
		},

		// 404
		{
			request: &testingRequest{
				method:     http.MethodPut,
				url:        fmt.Sprintf("%s/%d", labelAPIBasePath, 10000),
				credential: nonSysAdmin,
			},
			code: http.StatusNotFound,
		},

		// 403 non-member user
		{
			request: &testingRequest{
				method:     http.MethodPut,
				url:        fmt.Sprintf("%s/%d", labelAPIBasePath, labelID),
				credential: nonSysAdmin,
			},
			code: http.StatusForbidden,
		},

		// 403 developer
		{
			request: &testingRequest{
				method:     http.MethodPut,
				url:        fmt.Sprintf("%s/%d", labelAPIBasePath, labelID),
				credential: projDeveloper,
			},
			code: http.StatusForbidden,
		},

		// 400
		{
			request: &testingRequest{
				method: http.MethodPut,
				url:    fmt.Sprintf("%s/%d", labelAPIBasePath, labelID),
				bodyJSON: &models.Label{
					Name:      "",
					Scope:     common.LabelScopeProject,
					ProjectID: 1,
				},
				credential: projAdmin,
			},
			code: http.StatusBadRequest,
		},

		// 200
		{
			request: &testingRequest{
				method: http.MethodPut,
				url:    fmt.Sprintf("%s/%d", labelAPIBasePath, labelID),
				bodyJSON: &models.Label{
					Name:      "product",
					Scope:     common.LabelScopeProject,
					ProjectID: 1,
				},
				credential: projAdmin,
			},
			code: http.StatusOK,
		},
	}

	runCodeCheckingCases(t, cases...)

	label := &models.Label{}
	err := handleAndParse(&testingRequest{
		method: http.MethodGet,
		url:    fmt.Sprintf("%s/%d", labelAPIBasePath, labelID),
	}, label)
	require.Nil(t, err)
	assert.Equal(t, "product", label.Name)
}

func TestLabelAPIDelete(t *testing.T) {
	cases := []*codeCheckingCase{
		// 401
		{
			request: &testingRequest{
				method: http.MethodDelete,
				url:    fmt.Sprintf("%s/%d", labelAPIBasePath, labelID),
			},
			code: http.StatusUnauthorized,
		},

		// 400
		{
			request: &testingRequest{
				method:     http.MethodDelete,
				url:        fmt.Sprintf("%s/%d", labelAPIBasePath, 0),
				credential: nonSysAdmin,
			},
			code: http.StatusBadRequest,
		},

		// 404
		{
			request: &testingRequest{
				method:     http.MethodDelete,
				url:        fmt.Sprintf("%s/%d", labelAPIBasePath, 10000),
				credential: nonSysAdmin,
			},
			code: http.StatusNotFound,
		},

		// 403 non-member user
		{
			request: &testingRequest{
				method:     http.MethodDelete,
				url:        fmt.Sprintf("%s/%d", labelAPIBasePath, labelID),
				credential: nonSysAdmin,
			},
			code: http.StatusForbidden,
		},

		// 403 developer
		{
			request: &testingRequest{
				method:     http.MethodDelete,
				url:        fmt.Sprintf("%s/%d", labelAPIBasePath, labelID),
				credential: projDeveloper,
			},
			code: http.StatusForbidden,
		},

		// 200
		{
			request: &testingRequest{
				method:     http.MethodDelete,
				url:        fmt.Sprintf("%s/%d", labelAPIBasePath, labelID),
				credential: projAdmin,
			},
			code: http.StatusOK,
		},

		// 404
		{
			request: &testingRequest{
				method:     http.MethodDelete,
				url:        fmt.Sprintf("%s/%d", labelAPIBasePath, labelID),
				credential: projAdmin,
			},
			code: http.StatusNotFound,
		},
	}

	runCodeCheckingCases(t, cases...)
}

func TestListResources(t *testing.T) {
	// global level label
	globalLabelID, err := dao.AddLabel(&models.Label{
		Name:  "globel_level_label",
		Scope: common.LabelScopeGlobal,
	})
	require.Nil(t, err)
	defer dao.DeleteLabel(globalLabelID)

	// project level label
	projectLabelID, err := dao.AddLabel(&models.Label{
		Name:      "project_level_label",
		Scope:     common.LabelScopeProject,
		ProjectID: 1,
	})
	require.Nil(t, err)
	defer dao.DeleteLabel(projectLabelID)

	registryID, err := rep_dao.AddRegistry(&dao_models.Registry{
		Name: "target_for_testing_label_resource",
		URL:  "https://192.168.0.1",
	})
	require.Nil(t, err)
	defer rep_dao.DeleteRegistry(registryID)

	// create a policy references both global and project labels
	policyID, err := dao.AddRepPolicy(models.RepPolicy{
		Name:      "policy_for_testing_label_resource",
		ProjectID: 1,
		TargetID:  registryID,
		Trigger:   fmt.Sprintf(`{"kind":"%s"}`, replication.TriggerKindManual),
		Filters: fmt.Sprintf(`[{"kind":"%s","value":%d}, {"kind":"%s","value":%d}]`,
			replication.FilterItemKindLabel, globalLabelID,
			replication.FilterItemKindLabel, projectLabelID),
	})
	require.Nil(t, err)
	defer dao.DeleteRepPolicy(policyID)

	cases := []*codeCheckingCase{
		// 401
		{
			request: &testingRequest{
				method: http.MethodGet,
				url:    fmt.Sprintf("%s/%d/resources", labelAPIBasePath, globalLabelID),
			},
			code: http.StatusUnauthorized,
		},
		// 404
		{
			request: &testingRequest{
				method:     http.MethodGet,
				url:        fmt.Sprintf("%s/%d/resources", labelAPIBasePath, 10000),
				credential: sysAdmin,
			},
			code: http.StatusNotFound,
		},
		// 403: global level label
		{
			request: &testingRequest{
				method:     http.MethodGet,
				url:        fmt.Sprintf("%s/%d/resources", labelAPIBasePath, globalLabelID),
				credential: projAdmin,
			},
			code: http.StatusForbidden,
		},
		// 403: project level label
		{
			request: &testingRequest{
				method:     http.MethodGet,
				url:        fmt.Sprintf("%s/%d/resources", labelAPIBasePath, projectLabelID),
				credential: projDeveloper,
			},
			code: http.StatusForbidden,
		},
	}
	runCodeCheckingCases(t, cases...)

	// 200: global level label
	resources := map[string][]rep_models.ReplicationPolicy{}
	err = handleAndParse(&testingRequest{
		method:     http.MethodGet,
		url:        fmt.Sprintf("%s/%d/resources", labelAPIBasePath, globalLabelID),
		credential: sysAdmin,
	}, &resources)
	require.Nil(t, err)
	policies := resources["replication_policies"]
	require.Equal(t, 1, len(policies))
	assert.Equal(t, policyID, policies[0].ID)

	// 200: project level label
	resources = map[string][]rep_models.ReplicationPolicy{}
	err = handleAndParse(&testingRequest{
		method:     http.MethodGet,
		url:        fmt.Sprintf("%s/%d/resources", labelAPIBasePath, projectLabelID),
		credential: projAdmin,
	}, &resources)
	require.Nil(t, err)
	policies = resources["replication_policies"]
	require.Equal(t, 1, len(policies))
	assert.Equal(t, policyID, policies[0].ID)
}
