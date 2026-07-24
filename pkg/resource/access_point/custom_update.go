// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may
// not use this file except in compliance with the License. A copy of the
// License is located at
//
//     http://aws.amazon.com/apache2.0/
//
// or in the "license" file accompanying this file. This file is distributed
// on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either
// express or implied. See the License for the specific language governing
// permissions and limitations under the License.

package access_point

import (
	"context"
	"errors"

	ackcompare "github.com/aws-controllers-k8s/runtime/pkg/compare"
	svcsdk "github.com/aws/aws-sdk-go-v2/service/s3control"
	smithy "github.com/aws/smithy-go"
)

// customUpdate handles updates for the AccessPoint resource.
// Since the AccessPoint API has no UpdateAccessPoint operation for spec fields,
// this method only handles policy synchronisation via PutAccessPointPolicy /
// DeleteAccessPointPolicy.
func (rm *resourceManager) customUpdate(
	ctx context.Context,
	desired *resource,
	latest *resource,
	delta *ackcompare.Delta,
) (updated *resource, err error) {
	if delta.DifferentAt("Spec.Policy") {
		if desired.ko.Spec.Policy != nil {
			policyInput := &svcsdk.PutAccessPointPolicyInput{
				AccountId: desired.ko.Spec.AccountID,
				Name:      desired.ko.Spec.Name,
				Policy:    desired.ko.Spec.Policy,
			}
			_, err = rm.sdkapi.PutAccessPointPolicy(ctx, policyInput)
			rm.metrics.RecordAPICall("UPDATE", "PutAccessPointPolicy", err)
			if err != nil {
				return nil, err
			}
		} else {
			deleteInput := &svcsdk.DeleteAccessPointPolicyInput{
				AccountId: desired.ko.Spec.AccountID,
				Name:      desired.ko.Spec.Name,
			}
			_, err = rm.sdkapi.DeleteAccessPointPolicy(ctx, deleteInput)
			rm.metrics.RecordAPICall("UPDATE", "DeleteAccessPointPolicy", err)
			if err != nil {
				var awsErr smithy.APIError
				if !errors.As(err, &awsErr) || awsErr.ErrorCode() != "NoSuchAccessPointPolicy" {
					return nil, err
				}
			}
		}
	}

	ko := desired.ko.DeepCopy()
	rm.setStatusDefaults(ko)
	return &resource{ko}, nil
}
