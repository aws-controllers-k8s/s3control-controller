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

	ackcompare "github.com/aws-controllers-k8s/runtime/pkg/compare"
	ackrtlog "github.com/aws-controllers-k8s/runtime/pkg/runtime/log"
	"github.com/aws/aws-sdk-go-v2/aws"
	svcsdk "github.com/aws/aws-sdk-go-v2/service/s3control"
	svcsdktypes "github.com/aws/aws-sdk-go-v2/service/s3control/types"

	svcapitypes "github.com/aws-controllers-k8s/s3control-controller/apis/v1alpha1"
)

// getTags lists the tags on the access point.
func (rm *resourceManager) getTags(
	ctx context.Context,
	accountID *string,
	resourceARN string,
) (tags []*svcapitypes.Tag, err error) {
	rlog := ackrtlog.FromContext(ctx)
	exit := rlog.Trace("rm.getTags")
	defer func() { exit(err) }()

	resp, err := rm.sdkapi.ListTagsForResource(ctx, &svcsdk.ListTagsForResourceInput{
		AccountId:   accountID,
		ResourceArn: aws.String(resourceARN),
	})
	rm.metrics.RecordAPICall("GET", "ListTagsForResource", err)
	if err != nil {
		return nil, err
	}
	for _, tag := range resp.Tags {
		tags = append(tags, &svcapitypes.Tag{Key: tag.Key, Value: tag.Value})
	}
	return tags, nil
}

// syncTags adds and removes tags so the access point carries the desired set.
func (rm *resourceManager) syncTags(
	ctx context.Context,
	desired *resource,
	latest *resource,
) (err error) {
	rlog := ackrtlog.FromContext(ctx)
	exit := rlog.Trace("rm.syncTags")
	defer func() { exit(err) }()

	resourceARN := string(*latest.ko.Status.ACKResourceMetadata.ARN)
	from, _ := convertToOrderedACKTags(latest.ko.Spec.Tags)
	to, _ := convertToOrderedACKTags(desired.ko.Spec.Tags)
	added, _, removed := ackcompare.GetTagsDifference(from, to)

	if len(added) > 0 {
		toAdd := make([]svcsdktypes.Tag, 0, len(added))
		for key, value := range added {
			toAdd = append(toAdd, svcsdktypes.Tag{Key: aws.String(key), Value: aws.String(value)})
		}
		_, err = rm.sdkapi.TagResource(ctx, &svcsdk.TagResourceInput{
			AccountId:   desired.ko.Spec.AccountID,
			ResourceArn: aws.String(resourceARN),
			Tags:        toAdd,
		})
		rm.metrics.RecordAPICall("UPDATE", "TagResource", err)
		if err != nil {
			return err
		}
	}

	if len(removed) > 0 {
		toRemove := make([]string, 0, len(removed))
		for key := range removed {
			toRemove = append(toRemove, key)
		}
		_, err = rm.sdkapi.UntagResource(ctx, &svcsdk.UntagResourceInput{
			AccountId:   desired.ko.Spec.AccountID,
			ResourceArn: aws.String(resourceARN),
			TagKeys:     toRemove,
		})
		rm.metrics.RecordAPICall("UPDATE", "UntagResource", err)
		if err != nil {
			return err
		}
	}
	return nil
}
