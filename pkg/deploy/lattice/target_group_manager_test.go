package lattice

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"

	pkg_aws "github.com/aws/aws-application-networking-k8s/pkg/aws"
	mocks "github.com/aws/aws-application-networking-k8s/pkg/aws/services"
	"github.com/aws/aws-application-networking-k8s/pkg/config"
	"github.com/aws/aws-application-networking-k8s/pkg/model/core"
	model "github.com/aws/aws-application-networking-k8s/pkg/model/lattice"
	"github.com/aws/aws-application-networking-k8s/pkg/utils/gwlog"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/vpclattice"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

// target group does not exist, and is active after creation
func Test_CreateTargetGroup_TGNotExist_Active(t *testing.T) {
	c := gomock.NewController(t)
	defer c.Finish()
	ctx := context.TODO()
	mockLattice := mocks.NewMockLattice(c)
	cloud := pkg_aws.NewDefaultCloud(mockLattice, TestCloudConfig)

	tg_types := [2]string{"by-backendref", "by-serviceexport"}

	for _, tg_type := range tg_types {
		var tgSpec model.TargetGroupSpec

		if tg_type == "by-serviceexport" {
			// testing targetgroup for serviceexport
			tgSpec = model.TargetGroupSpec{
				Name: "test",
				Config: model.TargetGroupConfig{
					Port:                int32(8080),
					Protocol:            "HTTP",
					ProtocolVersion:     vpclattice.TargetGroupProtocolVersionHttp1,
					VpcID:               config.VpcID,
					EKSClusterName:      "",
					IsServiceImport:     false,
					IsServiceExport:     true,
					K8SServiceName:      "exportsvc1",
					K8SServiceNamespace: "default",
				},
			}
		} else if tg_type == "by-backendref" {
			// testing targetgroup for backendref
			tgSpec = model.TargetGroupSpec{
				Name: "test",
				Config: model.TargetGroupConfig{
					Port:                  int32(8080),
					Protocol:              "HTTP",
					ProtocolVersion:       vpclattice.TargetGroupProtocolVersionHttp1,
					VpcID:                 config.VpcID,
					EKSClusterName:        "",
					IsServiceImport:       false,
					IsServiceExport:       false,
					K8SServiceName:        "backend-svc1",
					K8SServiceNamespace:   "default",
					K8SHTTPRouteName:      "httproute1",
					K8SHTTPRouteNamespace: "default",
				},
			}
		}
		tgCreateInput := model.TargetGroup{
			ResourceMeta: core.ResourceMeta{},
			Spec:         tgSpec,
		}

		arn := "12345678912345678912"
		id := "12345678912345678912"
		name := "test-http-http1"
		tgStatus := vpclattice.TargetGroupStatusActive
		tgCreateOutput := &vpclattice.CreateTargetGroupOutput{
			Arn:    &arn,
			Id:     &id,
			Name:   &name,
			Status: &tgStatus,
		}
		p := int64(8080)
		emptystring := ""
		config := &vpclattice.TargetGroupConfig{
			Port:            &p,
			Protocol:        &tgSpec.Config.Protocol,
			VpcIdentifier:   &config.VpcID,
			ProtocolVersion: &tgSpec.Config.ProtocolVersion,
		}

		createTargetGroupInput := vpclattice.CreateTargetGroupInput{
			Config: config,
			Name:   &name,
			Type:   &emptystring,
			Tags:   cloud.DefaultTags(),
		}
		createTargetGroupInput.Tags[model.K8SServiceNameKey] = &tgSpec.Config.K8SServiceName
		createTargetGroupInput.Tags[model.K8SServiceNamespaceKey] = &tgSpec.Config.K8SServiceNamespace

		if tg_type == "by-serviceexport" {
			value := model.K8SServiceExportType
			createTargetGroupInput.Tags[model.K8SParentRefTypeKey] = &value
		} else if tg_type == "by-backendref" {
			value := model.K8SHTTPRouteType
			createTargetGroupInput.Tags[model.K8SParentRefTypeKey] = &value
			createTargetGroupInput.Tags[model.K8SHTTPRouteNameKey] = &tgSpec.Config.K8SHTTPRouteName
			createTargetGroupInput.Tags[model.K8SHTTPRouteNamespaceKey] = &tgSpec.Config.K8SHTTPRouteNamespace
		}

		listTgOutput := []*vpclattice.TargetGroupSummary{}

		mockLattice.EXPECT().ListTargetGroupsAsList(ctx, gomock.Any()).Return(listTgOutput, nil)
		mockLattice.EXPECT().CreateTargetGroupWithContext(ctx, &createTargetGroupInput).Return(tgCreateOutput, nil)
		tgManager := NewTargetGroupManager(gwlog.FallbackLogger, cloud)
		resp, err := tgManager.Create(ctx, &tgCreateInput)

		assert.Nil(t, err)
		assert.Equal(t, resp.TargetGroupARN, arn)
		assert.Equal(t, resp.TargetGroupID, id)
	}
}

// target group status is failed, and is active after creation
func Test_CreateTargetGroup_TGFailed_Active(t *testing.T) {
	c := gomock.NewController(t)
	defer c.Finish()
	ctx := context.TODO()
	mockLattice := mocks.NewMockLattice(c)
	cloud := pkg_aws.NewDefaultCloud(mockLattice, TestCloudConfig)

	tgSpec := model.TargetGroupSpec{
		Name:   "test",
		Config: model.TargetGroupConfig{},
	}
	tgCreateInput := model.TargetGroup{
		ResourceMeta: core.ResourceMeta{},
		Spec:         tgSpec,
		Status:       nil,
	}

	arn := "12345678912345678912"
	id := "12345678912345678912"
	name := "test"
	tgStatus := vpclattice.TargetGroupStatusActive
	tgCreateOutput := &vpclattice.CreateTargetGroupOutput{
		Arn:    &arn,
		Id:     &id,
		Name:   &name,
		Status: &tgStatus,
	}

	beforeCreateStatus := vpclattice.TargetGroupStatusCreateFailed
	tgSummary := vpclattice.TargetGroupSummary{
		Arn:    &arn,
		Id:     &id,
		Name:   &name,
		Status: &beforeCreateStatus,
	}
	listTgOutput := []*vpclattice.TargetGroupSummary{&tgSummary}

	mockLattice.EXPECT().ListTargetGroupsAsList(ctx, gomock.Any()).Return(listTgOutput, nil)
	mockLattice.EXPECT().CreateTargetGroupWithContext(ctx, gomock.Any()).Return(tgCreateOutput, nil)

	tgManager := NewTargetGroupManager(gwlog.FallbackLogger, cloud)
	resp, err := tgManager.Create(ctx, &tgCreateInput)

	assert.Nil(t, err)
	assert.Equal(t, resp.TargetGroupARN, arn)
	assert.Equal(t, resp.TargetGroupID, id)
}

// target group status is active before creation, no need to recreate
func Test_CreateTargetGroup_TGActive_UpdateHealthCheck(t *testing.T) {
	tests := []struct {
		healthCheckConfig *vpclattice.HealthCheckConfig
		wantErr           bool
	}{
		{
			healthCheckConfig: &vpclattice.HealthCheckConfig{
				Enabled: aws.Bool(false),
			},
			wantErr: false,
		},
		{
			healthCheckConfig: nil,
			wantErr:           false,
		},
		{
			wantErr: true,
		},
	}

	ctx := context.TODO()

	arn := "12345678912345678912"
	id := "12345678912345678912"

	for i, test := range tests {
		t.Run(fmt.Sprintf("Test_%d", i), func(t *testing.T) {
			c := gomock.NewController(t)
			defer c.Finish()

			mockLattice := mocks.NewMockLattice(c)
			cloud := pkg_aws.NewDefaultCloud(mockLattice, TestCloudConfig)
			tgManager := NewTargetGroupManager(gwlog.FallbackLogger, cloud)

			tgSpec := model.TargetGroupSpec{
				Name: "test",
				Config: model.TargetGroupConfig{
					Protocol:          vpclattice.TargetGroupProtocolHttps,
					ProtocolVersion:   vpclattice.TargetGroupProtocolVersionHttp1,
					HealthCheckConfig: test.healthCheckConfig,
				},
			}

			tgCreateInput := model.TargetGroup{
				ResourceMeta: core.ResourceMeta{},
				Spec:         tgSpec,
			}

			tgSummary := vpclattice.TargetGroupSummary{
				Arn:    &arn,
				Id:     &id,
				Name:   aws.String("test-https-http1"),
				Status: aws.String(vpclattice.TargetGroupStatusActive),
				Port:   aws.Int64(80),
			}

			listTgOutput := []*vpclattice.TargetGroupSummary{&tgSummary}

			mockLattice.EXPECT().ListTargetGroupsAsList(ctx, gomock.Any()).Return(listTgOutput, nil)

			if test.wantErr {
				mockLattice.EXPECT().UpdateTargetGroupWithContext(ctx, gomock.Any()).Return(nil, errors.New("error"))
			} else {
				mockLattice.EXPECT().UpdateTargetGroupWithContext(ctx, gomock.Any()).Return(nil, nil)
			}

			resp, err := tgManager.Create(ctx, &tgCreateInput)

			if test.wantErr {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
				assert.Equal(t, resp.TargetGroupARN, arn)
				assert.Equal(t, resp.TargetGroupID, id)
			}
		})
	}
}

// target group status is create-in-progress before creation, return Retry
func Test_CreateTargetGroup_TGCreateInProgress_Retry(t *testing.T) {
	c := gomock.NewController(t)
	defer c.Finish()
	ctx := context.TODO()
	mockLattice := mocks.NewMockLattice(c)
	cloud := pkg_aws.NewDefaultCloud(mockLattice, TestCloudConfig)

	tgSpec := model.TargetGroupSpec{
		Name:   "test",
		Config: model.TargetGroupConfig{},
	}
	tgCreateInput := model.TargetGroup{
		ResourceMeta: core.ResourceMeta{},
		Spec:         tgSpec,
	}

	arn := "12345678912345678912"
	id := "12345678912345678912"
	name := "test"

	beforeCreateStatus := vpclattice.TargetGroupStatusCreateInProgress
	tgSummary := vpclattice.TargetGroupSummary{
		Arn:    &arn,
		Id:     &id,
		Name:   &name,
		Status: &beforeCreateStatus,
	}
	listTgOutput := []*vpclattice.TargetGroupSummary{&tgSummary}

	mockLattice.EXPECT().ListTargetGroupsAsList(ctx, gomock.Any()).Return(listTgOutput, errors.New(LATTICE_RETRY))
	tgManager := NewTargetGroupManager(gwlog.FallbackLogger, cloud)
	resp, err := tgManager.Create(ctx, &tgCreateInput)

	assert.NotNil(t, err)
	assert.Equal(t, err, errors.New(LATTICE_RETRY))
	assert.Equal(t, resp.TargetGroupARN, "")
	assert.Equal(t, resp.TargetGroupID, "")
}

// target group status is delete-in-progress before creation, return Retry
func Test_CreateTargetGroup_TGDeleteInProgress_Retry(t *testing.T) {
	c := gomock.NewController(t)
	defer c.Finish()
	ctx := context.TODO()
	mockLattice := mocks.NewMockLattice(c)
	cloud := pkg_aws.NewDefaultCloud(mockLattice, TestCloudConfig)

	tgSpec := model.TargetGroupSpec{
		Name:   "test",
		Config: model.TargetGroupConfig{},
	}
	tgCreateInput := model.TargetGroup{
		ResourceMeta: core.ResourceMeta{},
		Spec:         tgSpec,
	}

	arn := "12345678912345678912"
	id := "12345678912345678912"
	name := "test"

	beforeCreateStatus := vpclattice.TargetGroupStatusDeleteInProgress
	tgSummary := vpclattice.TargetGroupSummary{
		Arn:    &arn,
		Id:     &id,
		Name:   &name,
		Status: &beforeCreateStatus,
	}
	listTgOutput := []*vpclattice.TargetGroupSummary{&tgSummary}

	mockLattice.EXPECT().ListTargetGroupsAsList(ctx, gomock.Any()).Return(listTgOutput, errors.New(LATTICE_RETRY))
	tgManager := NewTargetGroupManager(gwlog.FallbackLogger, cloud)
	resp, err := tgManager.Create(ctx, &tgCreateInput)

	assert.NotNil(t, err)
	assert.Equal(t, err, errors.New(LATTICE_RETRY))
	assert.Equal(t, resp.TargetGroupARN, "")
	assert.Equal(t, resp.TargetGroupID, "")
}

// target group is not in-progress before, get create-in-progress, should return retry
func Test_CreateTargetGroup_TGNotExist_CreateInProgress(t *testing.T) {
	c := gomock.NewController(t)
	defer c.Finish()
	ctx := context.TODO()
	mockLattice := mocks.NewMockLattice(c)
	cloud := pkg_aws.NewDefaultCloud(mockLattice, TestCloudConfig)

	tgSpec := model.TargetGroupSpec{
		Name:   "test",
		Config: model.TargetGroupConfig{},
	}
	tgCreateInput := model.TargetGroup{
		ResourceMeta: core.ResourceMeta{},
		Spec:         tgSpec,
	}

	arn := "12345678912345678912"
	id := "12345678912345678912"
	name := "test"
	tgStatus := vpclattice.TargetGroupStatusCreateInProgress
	tgCreateOutput := &vpclattice.CreateTargetGroupOutput{
		Arn:    &arn,
		Id:     &id,
		Name:   &name,
		Status: &tgStatus,
	}

	listTgOutput := []*vpclattice.TargetGroupSummary{}

	mockLattice.EXPECT().ListTargetGroupsAsList(ctx, gomock.Any()).Return(listTgOutput, nil)
	mockLattice.EXPECT().CreateTargetGroupWithContext(ctx, gomock.Any()).Return(tgCreateOutput, nil)

	tgManager := NewTargetGroupManager(gwlog.FallbackLogger, cloud)
	resp, err := tgManager.Create(ctx, &tgCreateInput)

	assert.NotNil(t, err)
	assert.NotNil(t, err, errors.New(LATTICE_RETRY))
	assert.Equal(t, resp.TargetGroupARN, "")
	assert.Equal(t, resp.TargetGroupID, "")
}

// target group is not in-progress before, get delete-in-progress, should return retry
func Test_CreateTargetGroup_TGNotExist_DeleteInProgress(t *testing.T) {
	c := gomock.NewController(t)
	defer c.Finish()
	ctx := context.TODO()
	mockLattice := mocks.NewMockLattice(c)
	cloud := pkg_aws.NewDefaultCloud(mockLattice, TestCloudConfig)

	tgSpec := model.TargetGroupSpec{
		Name:   "test",
		Config: model.TargetGroupConfig{},
	}
	tgCreateInput := model.TargetGroup{
		ResourceMeta: core.ResourceMeta{},
		Spec:         tgSpec,
	}

	arn := "12345678912345678912"
	id := "12345678912345678912"
	name := "test"
	tgStatus := vpclattice.TargetGroupStatusDeleteInProgress
	tgCreateOutput := &vpclattice.CreateTargetGroupOutput{
		Arn:    &arn,
		Id:     &id,
		Name:   &name,
		Status: &tgStatus,
	}

	listTgOutput := []*vpclattice.TargetGroupSummary{}

	mockLattice.EXPECT().ListTargetGroupsAsList(ctx, gomock.Any()).Return(listTgOutput, nil)
	mockLattice.EXPECT().CreateTargetGroupWithContext(ctx, gomock.Any()).Return(tgCreateOutput, nil)

	tgManager := NewTargetGroupManager(gwlog.FallbackLogger, cloud)
	resp, err := tgManager.Create(ctx, &tgCreateInput)

	assert.NotNil(t, err)
	assert.NotNil(t, err, errors.New(LATTICE_RETRY))
	assert.Equal(t, resp.TargetGroupARN, "")
	assert.Equal(t, resp.TargetGroupID, "")
}

// target group is not in-progress before, get failed, should return retry
func Test_CreateTargetGroup_TGNotExist_Failed(t *testing.T) {
	c := gomock.NewController(t)
	defer c.Finish()
	ctx := context.TODO()
	mockLattice := mocks.NewMockLattice(c)
	cloud := pkg_aws.NewDefaultCloud(mockLattice, TestCloudConfig)

	tgSpec := model.TargetGroupSpec{
		Name:   "test",
		Config: model.TargetGroupConfig{},
	}
	tgCreateInput := model.TargetGroup{
		ResourceMeta: core.ResourceMeta{},
		Spec:         tgSpec,
	}

	arn := "12345678912345678912"
	id := "12345678912345678912"
	name := "test"
	tgStatus := vpclattice.TargetGroupStatusCreateFailed
	tgCreateOutput := &vpclattice.CreateTargetGroupOutput{
		Arn:    &arn,
		Id:     &id,
		Name:   &name,
		Status: &tgStatus,
	}

	listTgOutput := []*vpclattice.TargetGroupSummary{}

	mockLattice.EXPECT().ListTargetGroupsAsList(ctx, gomock.Any()).Return(listTgOutput, nil)
	mockLattice.EXPECT().CreateTargetGroupWithContext(ctx, gomock.Any()).Return(tgCreateOutput, nil)

	tgManager := NewTargetGroupManager(gwlog.FallbackLogger, cloud)
	resp, err := tgManager.Create(ctx, &tgCreateInput)

	assert.NotNil(t, err)
	assert.NotNil(t, err, errors.New(LATTICE_RETRY))
	assert.Equal(t, resp.TargetGroupARN, "")
	assert.Equal(t, resp.TargetGroupID, "")
}

// Failed to list target group, should return error
func Test_CreateTargetGroup_ListTGError(t *testing.T) {
	c := gomock.NewController(t)
	defer c.Finish()
	ctx := context.TODO()
	mockLattice := mocks.NewMockLattice(c)
	cloud := pkg_aws.NewDefaultCloud(mockLattice, TestCloudConfig)

	tgSpec := model.TargetGroupSpec{
		Name:   "test",
		Config: model.TargetGroupConfig{},
	}
	tgCreateInput := model.TargetGroup{
		ResourceMeta: core.ResourceMeta{},
		Spec:         tgSpec,
	}

	listTgOutput := []*vpclattice.TargetGroupSummary{}

	mockLattice.EXPECT().ListTargetGroupsAsList(ctx, gomock.Any()).Return(listTgOutput, errors.New("test"))

	tgManager := NewTargetGroupManager(gwlog.FallbackLogger, cloud)
	resp, err := tgManager.Create(ctx, &tgCreateInput)

	assert.NotNil(t, err)
	assert.NotNil(t, err, errors.New("test"))
	assert.Equal(t, resp.TargetGroupARN, "")
	assert.Equal(t, resp.TargetGroupID, "")
}

// Failed to create target group, should return error
func Test_CreateTargetGroup_CreateTGFailed(t *testing.T) {
	c := gomock.NewController(t)
	defer c.Finish()
	ctx := context.TODO()
	mockLattice := mocks.NewMockLattice(c)
	cloud := pkg_aws.NewDefaultCloud(mockLattice, TestCloudConfig)

	tgSpec := model.TargetGroupSpec{
		Name:   "test",
		Config: model.TargetGroupConfig{},
	}
	tgCreateInput := model.TargetGroup{
		ResourceMeta: core.ResourceMeta{},
		Spec:         tgSpec,
	}

	arn := "12345678912345678912"
	id := "12345678912345678912"
	name := "test"
	tgStatus := vpclattice.TargetGroupStatusCreateFailed
	tgCreateOutput := &vpclattice.CreateTargetGroupOutput{
		Arn:    &arn,
		Id:     &id,
		Name:   &name,
		Status: &tgStatus,
	}

	listTgOutput := []*vpclattice.TargetGroupSummary{}

	mockLattice.EXPECT().ListTargetGroupsAsList(ctx, gomock.Any()).Return(listTgOutput, nil)
	mockLattice.EXPECT().CreateTargetGroupWithContext(ctx, gomock.Any()).Return(tgCreateOutput, errors.New("test"))
	tgManager := NewTargetGroupManager(gwlog.FallbackLogger, cloud)
	resp, err := tgManager.Create(ctx, &tgCreateInput)

	assert.NotNil(t, err)
	assert.NotNil(t, err, errors.New("test"))
	assert.Equal(t, resp.TargetGroupARN, "")
	assert.Equal(t, resp.TargetGroupID, "")
}

// Case1: Deregister unused status targets and delete target group work perfectly fine
// Case2: Delete target group that no targets register on it
// Case3: While deleting target group, deregister targets fails
// Case4: While deleting target group, list targets fails
// Case5: While deleting target group, deregister targets unsuccessfully
// Case6: Delete target group fails
// Case7: While deleting target group, that it has non-unused status targets, return LATTICE_RETRY
// Case8: While deleting target group, the vpcLatticeSess.DeleteTargetGroupWithContext() return ResourceNotFoundException, delete target group success and return err nil

// Case1: Deregister unused status targets and delete target group work perfectly fine
func Test_DeleteTG_DeRegisterTargets_DeleteTargetGroup(t *testing.T) {
	sId := "123.456.7.890"
	sPort := int64(80)
	targetStatus := vpclattice.TargetStatusUnused
	targetsList := &vpclattice.TargetSummary{
		Id:     &sId,
		Port:   &sPort,
		Status: &targetStatus,
	}
	targetsSuccessful := &vpclattice.Target{
		Id:   &sId,
		Port: &sPort,
	}

	listTargetsOutput := []*vpclattice.TargetSummary{targetsList}
	successful := []*vpclattice.Target{targetsSuccessful}
	deRegisterTargetsOutput := &vpclattice.DeregisterTargetsOutput{
		Successful: successful,
	}
	deleteTargetGroupOutput := &vpclattice.DeleteTargetGroupOutput{}

	tgSpec := model.TargetGroupSpec{
		Name:      "test",
		Config:    model.TargetGroupConfig{},
		Type:      "IP",
		IsDeleted: false,
		LatticeID: "123",
	}
	tgDeleteInput := model.TargetGroup{
		ResourceMeta: core.ResourceMeta{},
		Spec:         tgSpec,
	}
	c := gomock.NewController(t)
	defer c.Finish()
	ctx := context.TODO()
	mockLattice := mocks.NewMockLattice(c)
	cloud := pkg_aws.NewDefaultCloud(mockLattice, TestCloudConfig)

	mockLattice.EXPECT().ListTargetsAsList(ctx, gomock.Any()).Return(listTargetsOutput, nil)
	mockLattice.EXPECT().DeregisterTargetsWithContext(ctx, gomock.Any()).Return(deRegisterTargetsOutput, nil)
	mockLattice.EXPECT().DeleteTargetGroupWithContext(ctx, gomock.Any()).Return(deleteTargetGroupOutput, nil)

	tgManager := NewTargetGroupManager(gwlog.FallbackLogger, cloud)

	err := tgManager.Delete(ctx, &tgDeleteInput)

	assert.Nil(t, err)
}

// Case2: Delete target group that no targets register on it
func Test_DeleteTG_NoRegisteredTargets_DeleteTargetGroup(t *testing.T) {
	c := gomock.NewController(t)
	defer c.Finish()
	ctx := context.TODO()
	mockLattice := mocks.NewMockLattice(c)
	cloud := pkg_aws.NewDefaultCloud(mockLattice, TestCloudConfig)

	sId := "123.456.7.890"
	sPort := int64(80)
	targets := &vpclattice.TargetSummary{
		Id:   &sId,
		Port: &sPort,
	}
	listTargetsOutput := []*vpclattice.TargetSummary{targets}
	deRegisterTargetsOutput := &vpclattice.DeregisterTargetsOutput{}
	deleteTargetGroupOutput := &vpclattice.DeleteTargetGroupOutput{}

	tgSpec := model.TargetGroupSpec{
		Name:      "test",
		Config:    model.TargetGroupConfig{},
		Type:      "IP",
		IsDeleted: false,
		LatticeID: "123",
	}
	tgDeleteInput := model.TargetGroup{
		ResourceMeta: core.ResourceMeta{},
		Spec:         tgSpec,
	}
	mockLattice.EXPECT().ListTargetsAsList(ctx, gomock.Any()).Return(listTargetsOutput, nil)
	mockLattice.EXPECT().DeregisterTargetsWithContext(ctx, gomock.Any()).Return(deRegisterTargetsOutput, nil)
	mockLattice.EXPECT().DeleteTargetGroupWithContext(ctx, gomock.Any()).Return(deleteTargetGroupOutput, nil)

	tgManager := NewTargetGroupManager(gwlog.FallbackLogger, cloud)

	err := tgManager.Delete(ctx, &tgDeleteInput)

	assert.Nil(t, err)
}

// Case3: While deleting target group, deregister targets fails
func Test_DeleteTG_DeRegisteredTargetsFailed(t *testing.T) {
	c := gomock.NewController(t)
	defer c.Finish()
	ctx := context.TODO()
	mockLattice := mocks.NewMockLattice(c)
	cloud := pkg_aws.NewDefaultCloud(mockLattice, TestCloudConfig)

	sId := "123.456.7.890"
	sPort := int64(80)
	targets := &vpclattice.TargetSummary{
		Id:   &sId,
		Port: &sPort,
	}
	listTargetsOutput := []*vpclattice.TargetSummary{targets}
	deRegisterTargetsOutput := &vpclattice.DeregisterTargetsOutput{}

	tgSpec := model.TargetGroupSpec{
		Name:      "test",
		Config:    model.TargetGroupConfig{},
		Type:      "IP",
		IsDeleted: false,
		LatticeID: "123",
	}
	tgDeleteInput := model.TargetGroup{
		ResourceMeta: core.ResourceMeta{},
		Spec:         tgSpec,
	}
	mockLattice.EXPECT().ListTargetsAsList(ctx, gomock.Any()).Return(listTargetsOutput, nil)
	mockLattice.EXPECT().DeregisterTargetsWithContext(ctx, gomock.Any()).Return(deRegisterTargetsOutput, errors.New("Deregister_failed"))

	tgManager := NewTargetGroupManager(gwlog.FallbackLogger, cloud)

	err := tgManager.Delete(ctx, &tgDeleteInput)

	assert.NotNil(t, err)
	assert.Equal(t, err, errors.New("Deregister_failed"))
}

// Case4: While deleting target group, list targets fails
func Test_DeleteTG_ListTargetsFailed(t *testing.T) {
	c := gomock.NewController(t)
	defer c.Finish()
	ctx := context.TODO()
	mockLattice := mocks.NewMockLattice(c)
	cloud := pkg_aws.NewDefaultCloud(mockLattice, TestCloudConfig)

	sId := "123.456.7.890"
	sPort := int64(80)
	targets := &vpclattice.TargetSummary{
		Id:   &sId,
		Port: &sPort,
	}
	listTargetsOutput := []*vpclattice.TargetSummary{targets}

	tgSpec := model.TargetGroupSpec{
		Name:      "test",
		Config:    model.TargetGroupConfig{},
		Type:      "IP",
		IsDeleted: false,
		LatticeID: "123",
	}
	tgDeleteInput := model.TargetGroup{
		ResourceMeta: core.ResourceMeta{},
		Spec:         tgSpec,
		Status:       nil,
	}
	mockLattice.EXPECT().ListTargetsAsList(ctx, gomock.Any()).Return(listTargetsOutput, errors.New("Listregister_failed"))

	tgManager := NewTargetGroupManager(gwlog.FallbackLogger, cloud)

	err := tgManager.Delete(ctx, &tgDeleteInput)

	assert.NotNil(t, err)
	assert.Equal(t, err, errors.New("Listregister_failed"))
}

// Case5: While deleting target group, deregister targets unsuccessfully
func Test_DeleteTG_DeRegisterTargetsUnsuccessfully(t *testing.T) {
	c := gomock.NewController(t)
	defer c.Finish()
	ctx := context.TODO()
	mockLattice := mocks.NewMockLattice(c)
	cloud := pkg_aws.NewDefaultCloud(mockLattice, TestCloudConfig)

	sId := "123.456.7.890"
	sPort := int64(80)
	targets := &vpclattice.TargetSummary{
		Id:   &sId,
		Port: &sPort,
	}
	listTargetsOutput := []*vpclattice.TargetSummary{targets}
	targetsFailure := &vpclattice.TargetFailure{
		FailureCode:    nil,
		FailureMessage: nil,
		Id:             &sId,
		Port:           &sPort,
	}
	unsuccessful := []*vpclattice.TargetFailure{targetsFailure}
	deRegisterTargetsOutput := &vpclattice.DeregisterTargetsOutput{
		Unsuccessful: unsuccessful,
	}

	tgSpec := model.TargetGroupSpec{
		Name:      "test",
		Config:    model.TargetGroupConfig{},
		Type:      "IP",
		IsDeleted: false,
		LatticeID: "123",
	}
	tgDeleteInput := model.TargetGroup{
		ResourceMeta: core.ResourceMeta{},
		Spec:         tgSpec,
	}
	mockLattice.EXPECT().ListTargetsAsList(ctx, gomock.Any()).Return(listTargetsOutput, nil)
	mockLattice.EXPECT().DeregisterTargetsWithContext(ctx, gomock.Any()).Return(deRegisterTargetsOutput, nil)

	tgManager := NewTargetGroupManager(gwlog.FallbackLogger, cloud)

	err := tgManager.Delete(ctx, &tgDeleteInput)

	assert.NotNil(t, err)
	assert.Equal(t, err, errors.New(LATTICE_RETRY))
}

// Case6: Delete target group fails
func Test_DeleteTG_DeRegisterTargets_DeleteTargetGroupFailed(t *testing.T) {
	c := gomock.NewController(t)
	defer c.Finish()
	ctx := context.TODO()
	mockLattice := mocks.NewMockLattice(c)
	cloud := pkg_aws.NewDefaultCloud(mockLattice, TestCloudConfig)

	sId := "123.456.7.890"
	sPort := int64(80)
	targetsList := &vpclattice.TargetSummary{
		Id:   &sId,
		Port: &sPort,
	}
	targetsSuccessful := &vpclattice.Target{
		Id:   &sId,
		Port: &sPort,
	}

	listTargetsOutput := []*vpclattice.TargetSummary{targetsList}
	successful := []*vpclattice.Target{targetsSuccessful}
	deRegisterTargetsOutput := &vpclattice.DeregisterTargetsOutput{
		Successful: successful,
	}
	deleteTargetGroupOutput := &vpclattice.DeleteTargetGroupOutput{}

	tgSpec := model.TargetGroupSpec{
		Name:      "test",
		Config:    model.TargetGroupConfig{},
		Type:      "IP",
		IsDeleted: false,
		LatticeID: "123",
	}
	tgDeleteInput := model.TargetGroup{
		ResourceMeta: core.ResourceMeta{},
		Spec:         tgSpec,
	}
	mockLattice.EXPECT().ListTargetsAsList(ctx, gomock.Any()).Return(listTargetsOutput, nil)
	mockLattice.EXPECT().DeregisterTargetsWithContext(ctx, gomock.Any()).Return(deRegisterTargetsOutput, nil)
	mockLattice.EXPECT().DeleteTargetGroupWithContext(ctx, gomock.Any()).Return(deleteTargetGroupOutput, errors.New("DeleteTG_failed"))

	tgManager := NewTargetGroupManager(gwlog.FallbackLogger, cloud)

	err := tgManager.Delete(ctx, &tgDeleteInput)

	assert.NotNil(t, err)
	assert.Equal(t, err, errors.New("DeleteTG_failed"))
}

// Case7: While deleting target group, it has non-unused status targets, return LATTICE_RETRY
func Test_DeleteTG_TargetsNonUnused(t *testing.T) {
	c := gomock.NewController(t)
	defer c.Finish()
	ctx := context.TODO()
	mockLattice := mocks.NewMockLattice(c)
	cloud := pkg_aws.NewDefaultCloud(mockLattice, TestCloudConfig)

	targetId := "123.456.7.890"
	targetPort := int64(80)
	targetStatus := vpclattice.TargetStatusHealthy
	targets := &vpclattice.TargetSummary{
		Id:     &targetId,
		Port:   &targetPort,
		Status: &targetStatus,
	}
	listTargetsOutput := []*vpclattice.TargetSummary{targets}

	tgSpec := model.TargetGroupSpec{
		Name:      "test",
		Config:    model.TargetGroupConfig{},
		Type:      "IP",
		IsDeleted: false,
		LatticeID: "123",
	}
	tgDeleteInput := model.TargetGroup{
		ResourceMeta: core.ResourceMeta{},
		Spec:         tgSpec,
		Status:       nil,
	}
	mockLattice.EXPECT().ListTargetsAsList(ctx, gomock.Any()).Return(listTargetsOutput, nil)

	tgManager := NewTargetGroupManager(gwlog.FallbackLogger, cloud)

	err := tgManager.Delete(ctx, &tgDeleteInput)

	assert.NotNil(t, err)
	assert.Equal(t, err, errors.New(LATTICE_RETRY))
}

// Case8: While deleting target group, the vpcLatticeSess.DeleteTargetGroupWithContext() return ResourceNotFoundException, delete target group success and return err nil
func Test_DeleteTG_vpcLatticeSessReturnResourceNotFound_DeleteTargetGroupSuccessAndErrIsNil(t *testing.T) {
	c := gomock.NewController(t)
	defer c.Finish()
	ctx := context.TODO()
	mockLattice := mocks.NewMockLattice(c)
	cloud := pkg_aws.NewDefaultCloud(mockLattice, TestCloudConfig)

	sId := "123.456.7.890"
	sPort := int64(80)
	targets := &vpclattice.TargetSummary{
		Id:   &sId,
		Port: &sPort,
	}
	listTargetsOutput := []*vpclattice.TargetSummary{targets}
	deRegisterTargetsOutput := &vpclattice.DeregisterTargetsOutput{}
	deleteTargetGroupOutput := &vpclattice.DeleteTargetGroupOutput{}

	tgSpec := model.TargetGroupSpec{
		Name:      "test",
		Config:    model.TargetGroupConfig{},
		Type:      "IP",
		IsDeleted: false,
		LatticeID: "123",
	}
	tgDeleteInput := model.TargetGroup{
		ResourceMeta: core.ResourceMeta{},
		Spec:         tgSpec,
	}
	mockLattice.EXPECT().ListTargetsAsList(ctx, gomock.Any()).Return(listTargetsOutput, nil)
	mockLattice.EXPECT().DeregisterTargetsWithContext(ctx, gomock.Any()).Return(deRegisterTargetsOutput, nil)
	mockLattice.EXPECT().DeleteTargetGroupWithContext(ctx, gomock.Any()).Return(deleteTargetGroupOutput, awserr.New(vpclattice.ErrCodeResourceNotFoundException, "", nil))

	tgManager := NewTargetGroupManager(gwlog.FallbackLogger, cloud)

	err := tgManager.Delete(ctx, &tgDeleteInput)

	assert.Nil(t, err)
}

func Test_ListTG_TGsExist(t *testing.T) {
	c := gomock.NewController(t)
	defer c.Finish()
	ctx := context.TODO()
	mockLattice := mocks.NewMockLattice(c)
	cloud := pkg_aws.NewDefaultCloud(mockLattice, TestCloudConfig)

	arn := "123456789"
	id := "123456789"
	name1 := "test1"
	tg1 := &vpclattice.TargetGroupSummary{
		Arn:  &arn,
		Id:   &id,
		Name: &name1,
	}
	name2 := "test2"
	tg2 := &vpclattice.TargetGroupSummary{
		Arn:  &arn,
		Id:   &id,
		Name: &name2,
	}
	listTGOutput := []*vpclattice.TargetGroupSummary{tg1, tg2}

	config1 := &vpclattice.TargetGroupConfig{
		VpcIdentifier: &config.VpcID,
	}
	getTG1 := &vpclattice.GetTargetGroupOutput{
		Config: config1,
	}

	vpcid2 := "123456789"
	config2 := &vpclattice.TargetGroupConfig{
		VpcIdentifier: &vpcid2,
	}
	getTG2 := &vpclattice.GetTargetGroupOutput{
		Config: config2,
	}

	mockLattice.EXPECT().ListTargetGroupsAsList(ctx, gomock.Any()).Return(listTGOutput, nil)
	mockLattice.EXPECT().GetTargetGroupWithContext(ctx, gomock.Any()).Return(getTG1, nil)
	// assume no tags
	mockLattice.EXPECT().ListTagsForResourceWithContext(ctx, gomock.Any()).Return(nil, errors.New("no tags"))
	mockLattice.EXPECT().GetTargetGroupWithContext(ctx, gomock.Any()).Return(getTG2, nil)

	tgManager := NewTargetGroupManager(gwlog.FallbackLogger, cloud)
	tgList, err := tgManager.List(ctx)
	expect := []targetGroupOutput{
		{
			getTargetGroupOutput: *getTG1,
			targetGroupTags:      nil,
		},
	}

	assert.Nil(t, err)
	assert.Equal(t, tgList, expect)
}

func Test_ListTG_NoTG(t *testing.T) {
	c := gomock.NewController(t)
	defer c.Finish()
	ctx := context.TODO()
	mockLattice := mocks.NewMockLattice(c)
	cloud := pkg_aws.NewDefaultCloud(mockLattice, TestCloudConfig)

	listTGOutput := []*vpclattice.TargetGroupSummary{}

	mockLattice.EXPECT().ListTargetGroupsAsList(ctx, gomock.Any()).Return(listTGOutput, nil)

	tgManager := NewTargetGroupManager(gwlog.FallbackLogger, cloud)
	tgList, err := tgManager.List(ctx)
	expectTgList := []targetGroupOutput(nil)

	assert.Nil(t, err)
	assert.Equal(t, tgList, expectTgList)
}

func Test_Get(t *testing.T) {
	tests := []struct {
		wantErr        error
		tgId           string
		tgArn          string
		tgName         string
		input          *model.TargetGroup
		wantOutput     model.TargetGroupStatus
		randomArn      string
		randomId       string
		randomName     string
		tgStatus       string
		tgStatusFailed string
	}{
		{
			wantErr: nil,
			tgId:    "tg-id-012345",
			tgArn:   "tg-arn-123456",
			tgName:  "tg-test-1-https-http1",
			input: &model.TargetGroup{
				ResourceMeta: core.ResourceMeta{},
				Spec: model.TargetGroupSpec{
					Name: "tg-test-1",
					Config: model.TargetGroupConfig{
						Protocol:        vpclattice.TargetGroupProtocolHttps,
						ProtocolVersion: vpclattice.TargetGroupProtocolVersionHttp1,
					},
					Type:      "",
					IsDeleted: false,
					LatticeID: "",
				},
				Status: nil,
			},
			wantOutput:     model.TargetGroupStatus{TargetGroupARN: "tg-arn-123456", TargetGroupID: "tg-id-012345"},
			randomArn:      "random-tg-arn-12345",
			randomId:       "random-tg-id-12345",
			randomName:     "tgrandom-1",
			tgStatus:       vpclattice.TargetGroupStatusActive,
			tgStatusFailed: vpclattice.TargetGroupStatusCreateFailed,
		},
		{
			wantErr: errors.New("Non existing Target Group"),
			tgId:    "tg-id-012345",
			tgArn:   "tg-arn-123456",
			tgName:  "tg-test-1-https-http1",
			input: &model.TargetGroup{
				ResourceMeta: core.ResourceMeta{},
				Spec: model.TargetGroupSpec{
					Name: "tg-test-1",
					Config: model.TargetGroupConfig{
						Protocol:        vpclattice.TargetGroupProtocolHttps,
						ProtocolVersion: vpclattice.TargetGroupProtocolVersionHttp1,
					},
					Type:      "",
					IsDeleted: false,
					LatticeID: "",
				},
				Status: nil,
			},
			wantOutput:     model.TargetGroupStatus{TargetGroupARN: "", TargetGroupID: ""},
			randomArn:      "random-tg-arn-12345",
			randomId:       "random-tg-id-12345",
			randomName:     "tgrandom-1",
			tgStatus:       vpclattice.TargetGroupStatusCreateFailed,
			tgStatusFailed: vpclattice.TargetGroupStatusCreateFailed,
		},
		{
			wantErr: errors.New(LATTICE_RETRY),
			tgId:    "tg-id-012345",
			tgArn:   "tg-arn-123456",
			tgName:  "tg-test-1-https-http1",
			input: &model.TargetGroup{
				ResourceMeta: core.ResourceMeta{},
				Spec: model.TargetGroupSpec{
					Name: "tg-test-1",
					Config: model.TargetGroupConfig{
						Protocol:        vpclattice.TargetGroupProtocolHttps,
						ProtocolVersion: vpclattice.TargetGroupProtocolVersionHttp1,
					},
					Type:      "",
					IsDeleted: false,
					LatticeID: "",
				},
				Status: nil,
			},
			wantOutput:     model.TargetGroupStatus{TargetGroupARN: "", TargetGroupID: ""},
			randomArn:      "random-tg-arn-12345",
			randomId:       "random-tg-id-12345",
			randomName:     "tgrandom-1",
			tgStatus:       vpclattice.TargetGroupStatusDeleteInProgress,
			tgStatusFailed: vpclattice.TargetGroupStatusDeleteFailed,
		},
		{
			wantErr: errors.New("Non existing Target Group"),
			tgId:    "tg-id-012345",
			tgArn:   "tg-arn-123456",
			tgName:  "tg-test-not-exist-https-http1",
			input: &model.TargetGroup{
				ResourceMeta: core.ResourceMeta{},
				Spec: model.TargetGroupSpec{
					Name: "tg-test-2",
					Config: model.TargetGroupConfig{
						Protocol:        vpclattice.TargetGroupProtocolHttps,
						ProtocolVersion: vpclattice.TargetGroupProtocolVersionHttp1,
					},
					Type:      "",
					IsDeleted: false,
					LatticeID: "",
				},
				Status: nil,
			},
			wantOutput:     model.TargetGroupStatus{TargetGroupARN: "", TargetGroupID: ""},
			randomArn:      "random-tg-arn-12345",
			randomId:       "random-tg-id-12345",
			randomName:     "tgrandom-2",
			tgStatus:       vpclattice.TargetGroupStatusCreateFailed,
			tgStatusFailed: vpclattice.TargetGroupStatusCreateFailed,
		},
	}

	for i, tt := range tests {
		t.Run(fmt.Sprintf("Test_%d", i), func(t *testing.T) {
			c := gomock.NewController(t)
			defer c.Finish()
			ctx := context.TODO()
			mockLattice := mocks.NewMockLattice(c)
			cloud := pkg_aws.NewDefaultCloud(mockLattice, TestCloudConfig)

			listTGinput := &vpclattice.ListTargetGroupsInput{}
			listTGOutput := []*vpclattice.TargetGroupSummary{
				{
					Arn:    &tt.randomArn,
					Id:     &tt.randomId,
					Name:   &tt.randomName,
					Status: &tt.tgStatusFailed,
					Type:   nil,
				},
				{
					Arn:    &tt.tgArn,
					Id:     &tt.tgId,
					Name:   &tt.tgName,
					Status: &tt.tgStatus,
					Type:   nil,
				}}

			mockLattice.EXPECT().ListTargetGroupsAsList(ctx, listTGinput).Return(listTGOutput, nil)

			targetGroupManager := NewTargetGroupManager(gwlog.FallbackLogger, cloud)

			resp, err := targetGroupManager.Get(ctx, tt.input)

			if tt.wantErr != nil {
				assert.NotNil(t, err)
				assert.Equal(t, err, tt.wantErr)
			} else {
				assert.Nil(t, err)
				assert.Equal(t, resp.TargetGroupID, tt.wantOutput.TargetGroupID)
				assert.Equal(t, resp.TargetGroupARN, tt.wantOutput.TargetGroupARN)
			}
		})
	}
}

func Test_defaultTargetGroupManager_getDefaultHealthCheckConfig(t *testing.T) {
	var (
		resetValue     = aws.Int64(0)
		defaultMatcher = &vpclattice.Matcher{
			HttpCode: aws.String("200"),
		}
		defaultPath     = aws.String("/")
		defaultProtocol = aws.String(vpclattice.TargetGroupProtocolHttp)
	)

	type args struct {
		targetGroupProtocolVersion string
	}

	tests := []struct {
		name string
		args args
		want *vpclattice.HealthCheckConfig
	}{
		{
			name: "HTTP1 default health check config",
			args: args{
				targetGroupProtocolVersion: vpclattice.TargetGroupProtocolVersionHttp1,
			},
			want: &vpclattice.HealthCheckConfig{
				Enabled:                    aws.Bool(true),
				HealthCheckIntervalSeconds: resetValue,
				HealthCheckTimeoutSeconds:  resetValue,
				HealthyThresholdCount:      resetValue,
				UnhealthyThresholdCount:    resetValue,
				Matcher:                    defaultMatcher,
				Path:                       defaultPath,
				Port:                       nil,
				Protocol:                   defaultProtocol,
				ProtocolVersion:            aws.String(vpclattice.HealthCheckProtocolVersionHttp1),
			},
		},
		{
			name: "empty target group protocol version default health check config",
			args: args{
				targetGroupProtocolVersion: "",
			},
			want: &vpclattice.HealthCheckConfig{
				Enabled:                    aws.Bool(true),
				HealthCheckIntervalSeconds: resetValue,
				HealthCheckTimeoutSeconds:  resetValue,
				HealthyThresholdCount:      resetValue,
				UnhealthyThresholdCount:    resetValue,
				Matcher:                    defaultMatcher,
				Path:                       defaultPath,
				Port:                       nil,
				Protocol:                   defaultProtocol,
				ProtocolVersion:            aws.String(vpclattice.HealthCheckProtocolVersionHttp1),
			},
		},
		{
			name: "HTTP2 default health check config",
			args: args{
				targetGroupProtocolVersion: vpclattice.TargetGroupProtocolVersionHttp2,
			},
			want: &vpclattice.HealthCheckConfig{
				Enabled:                    aws.Bool(false),
				HealthCheckIntervalSeconds: resetValue,
				HealthCheckTimeoutSeconds:  resetValue,
				HealthyThresholdCount:      resetValue,
				UnhealthyThresholdCount:    resetValue,
				Matcher:                    defaultMatcher,
				Path:                       defaultPath,
				Port:                       nil,
				Protocol:                   defaultProtocol,
				ProtocolVersion:            aws.String(vpclattice.HealthCheckProtocolVersionHttp2),
			},
		},
		{
			name: "GRPC default health check config",
			args: args{
				targetGroupProtocolVersion: vpclattice.TargetGroupProtocolVersionGrpc,
			},
			want: &vpclattice.HealthCheckConfig{
				Enabled:                    aws.Bool(false),
				HealthCheckIntervalSeconds: resetValue,
				HealthCheckTimeoutSeconds:  resetValue,
				HealthyThresholdCount:      resetValue,
				UnhealthyThresholdCount:    resetValue,
				Matcher:                    defaultMatcher,
				Path:                       defaultPath,
				Port:                       nil,
				Protocol:                   defaultProtocol,
				ProtocolVersion:            aws.String(vpclattice.HealthCheckProtocolVersionHttp1),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewTargetGroupManager(gwlog.FallbackLogger, nil)
			if got := s.getDefaultHealthCheckConfig(tt.args.targetGroupProtocolVersion); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("defaultTargetGroupManager.getDefaultHealthCheckConfig() = %v, want %v", got, tt.want)
			}
		})
	}
}
