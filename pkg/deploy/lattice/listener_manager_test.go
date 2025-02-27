package lattice

import (
	"context"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"k8s.io/apimachinery/pkg/types"

	"testing"

	"github.com/aws/aws-application-networking-k8s/pkg/model/core"
	"github.com/aws/aws-application-networking-k8s/pkg/utils/gwlog"

	"github.com/aws/aws-sdk-go/service/vpclattice"

	"github.com/aws/aws-application-networking-k8s/pkg/latticestore"

	pkg_aws "github.com/aws/aws-application-networking-k8s/pkg/aws"
	mocks "github.com/aws/aws-application-networking-k8s/pkg/aws/services"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	model "github.com/aws/aws-application-networking-k8s/pkg/model/lattice"
)

var namespaceName = types.NamespacedName{
	Namespace: "default",
	Name:      "test",
}
var listenerSummaries = []struct {
	Arn      string
	Id       string
	Name     string
	Port     int64
	Protocol string
}{
	{
		Arn:      "arn-1",
		Id:       "id-1",
		Name:     namespaceName.Name,
		Port:     80,
		Protocol: "HTTP",
	},
	{
		Arn:      "arn-2",
		Id:       "Id-2",
		Name:     namespaceName.Name,
		Port:     443,
		Protocol: "HTTPS",
	},
}
var summaries = []vpclattice.ListenerSummary{
	{
		Arn:      &listenerSummaries[0].Arn,
		Id:       &listenerSummaries[0].Id,
		Name:     &listenerSummaries[0].Name,
		Port:     &listenerSummaries[0].Port,
		Protocol: &listenerSummaries[0].Protocol,
	},
	{
		Arn:      &listenerSummaries[1].Arn,
		Id:       &listenerSummaries[1].Id,
		Name:     &listenerSummaries[1].Name,
		Port:     &listenerSummaries[1].Port,
		Protocol: &listenerSummaries[1].Protocol,
	},
}
var listenerList = vpclattice.ListListenersOutput{
	Items: []*vpclattice.ListenerSummary{
		&summaries[0],
		&summaries[1],
	},
}

func Test_AddListener(t *testing.T) {

	tests := []struct {
		name        string
		isUpdate    bool
		noServiceID bool
	}{
		{
			name:        "add listener",
			isUpdate:    false,
			noServiceID: false,
		},

		{
			name:        "update listener",
			isUpdate:    true,
			noServiceID: false,
		},

		{
			name:        "add listener, no service ID",
			isUpdate:    false,
			noServiceID: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := gomock.NewController(t)
			defer c.Finish()
			ctx := context.TODO()

			mockLattice := mocks.NewMockLattice(c)
			cloud := pkg_aws.NewDefaultCloud(mockLattice, TestCloudConfig)

			latticeDataStore := latticestore.NewLatticeDataStore()
			listenerManager := NewListenerManager(gwlog.FallbackLogger, cloud, latticeDataStore)

			var serviceID = "serviceID"
			var serviceARN = "serviceARN"
			var serviceDNS = "DNS-test"

			stack := core.NewDefaultStack(core.StackID(namespaceName))

			action := model.DefaultAction{
				BackendServiceName:      "tg-test",
				BackendServiceNamespace: "tg-default",
			}

			listenerResourceName := fmt.Sprintf("%s-%s-%d-%s", namespaceName.Name, namespaceName.Namespace,
				listenerSummaries[0].Port, "HTTP")

			listener := model.NewListener(stack, listenerResourceName, listenerSummaries[0].Port, "HTTP",
				namespaceName.Name, namespaceName.Namespace, action)

			if !tt.noServiceID {
				mockLattice.EXPECT().FindService(gomock.Any(), gomock.Any()).Return(
					&vpclattice.ServiceSummary{
						Name: aws.String((&ListenerLSNProvider{listener}).LatticeServiceName()),
						Arn:  aws.String(serviceARN),
						Id:   aws.String(serviceID),
						DnsEntry: &vpclattice.DnsEntry{
							DomainName:   aws.String(serviceDNS),
							HostedZoneId: aws.String("my-favourite-zone"),
						},
					}, nil).Times(1)
			} else {
				mockLattice.EXPECT().FindService(gomock.Any(), gomock.Any()).Return(nil, &mocks.NotFoundError{}).Times(1)
			}

			listenerOutput := vpclattice.CreateListenerOutput{}
			listenerInput := vpclattice.CreateListenerInput{}

			defaultStatus := aws.Int64(404)

			defaultResp := vpclattice.FixedResponseAction{
				StatusCode: defaultStatus,
			}
			defaultAction := vpclattice.RuleAction{
				FixedResponse: &defaultResp,
			}

			if !tt.noServiceID && !tt.isUpdate {
				listenerName := k8sLatticeListenerName(namespaceName.Name, namespaceName.Namespace,
					int(listenerSummaries[0].Port), listenerSummaries[0].Protocol)
				listenerInput = vpclattice.CreateListenerInput{
					DefaultAction:     &defaultAction,
					Name:              &listenerName,
					ServiceIdentifier: &serviceID,
					Protocol:          aws.String("HTTP"),
					Port:              aws.Int64(listenerSummaries[0].Port),
					Tags:              cloud.DefaultTags(),
				}
				listenerOutput = vpclattice.CreateListenerOutput{
					Arn:           &listenerSummaries[0].Arn,
					DefaultAction: &defaultAction,
					Id:            &listenerSummaries[0].Id,
				}
				mockLattice.EXPECT().CreateListener(&listenerInput).Return(&listenerOutput, nil)
			}

			if !tt.noServiceID {
				listenerListInput := vpclattice.ListListenersInput{
					ServiceIdentifier: aws.String(serviceID),
				}

				listenerOutput := vpclattice.ListListenersOutput{}

				if tt.isUpdate {
					listenerOutput = listenerList
				}

				mockLattice.EXPECT().ListListenersWithContext(ctx, &listenerListInput).Return(&listenerOutput, nil)
			}
			resp, err := listenerManager.Create(ctx, listener)

			if !tt.noServiceID {
				assert.NoError(t, err)

				assert.Equal(t, resp.ListenerARN, listenerSummaries[0].Arn)
				assert.Equal(t, resp.ListenerID, listenerSummaries[0].Id)
				assert.Equal(t, resp.Name, namespaceName.Name)
				assert.Equal(t, resp.Namespace, namespaceName.Namespace)
				assert.Equal(t, resp.Port, listenerSummaries[0].Port)
				assert.Equal(t, resp.Protocol, "HTTP")
			}

			fmt.Printf("listener create : resp %v, err %v, listenerOutput %v\n", resp, err, listenerOutput)

			if tt.noServiceID {
				assert.NotNil(t, err)
			}
		})
	}
}

func Test_ListListener(t *testing.T) {

	tests := []struct {
		Name   string
		mgrErr error
	}{
		{
			Name:   "listener LIST API call ok",
			mgrErr: nil,
		},
		{
			Name:   "listener List API call return NOK",
			mgrErr: errors.New("call failed"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			c := gomock.NewController(t)
			defer c.Finish()
			ctx := context.TODO()

			mockLattice := mocks.NewMockLattice(c)
			cloud := pkg_aws.NewDefaultCloud(mockLattice, pkg_aws.CloudConfig{})

			latticeDataStore := latticestore.NewLatticeDataStore()
			listenerManager := NewListenerManager(gwlog.FallbackLogger, cloud, latticeDataStore)

			serviceID := "service1-ID"
			listenerListInput := vpclattice.ListListenersInput{
				ServiceIdentifier: aws.String(serviceID),
			}
			mockLattice.EXPECT().ListListeners(&listenerListInput).Return(&listenerList, tt.mgrErr)

			resp, err := listenerManager.List(ctx, serviceID)
			fmt.Printf("listener list :%v, err: %v \n", resp, err)

			if err == nil {
				var i = 0
				for _, rsp := range resp {
					assert.Equal(t, *rsp.Arn, *listenerList.Items[i].Arn)
					i++
				}
			} else {
				assert.Equal(t, err, tt.mgrErr)
			}
		})
	}
}

func Test_DeleteListener(t *testing.T) {
	c := gomock.NewController(t)
	defer c.Finish()
	ctx := context.TODO()

	mockLattice := mocks.NewMockLattice(c)
	cloud := pkg_aws.NewDefaultCloud(mockLattice, TestCloudConfig)

	serviceID := "service1-ID"
	listenerID := "listener-ID"

	listenerDeleteInput := vpclattice.DeleteListenerInput{
		ServiceIdentifier:  aws.String(serviceID),
		ListenerIdentifier: aws.String(listenerID),
	}

	latticeDataStore := latticestore.NewLatticeDataStore()

	listenerDeleteOutput := vpclattice.DeleteListenerOutput{}
	mockLattice.EXPECT().DeleteListener(&listenerDeleteInput).Return(&listenerDeleteOutput, nil)

	listenerManager := NewListenerManager(gwlog.FallbackLogger, cloud, latticeDataStore)

	err := listenerManager.Delete(ctx, listenerID, serviceID)
	assert.Nil(t, err)
}
