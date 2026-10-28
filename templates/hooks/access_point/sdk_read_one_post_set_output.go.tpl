policyInput := &svcsdk.GetAccessPointPolicyInput{
	AccountId: r.ko.Spec.AccountID,
	Name:      r.ko.Spec.Name,
}
policyResp, policyErr := rm.sdkapi.GetAccessPointPolicy(ctx, policyInput)
rm.metrics.RecordAPICall("READ_ONE", "GetAccessPointPolicy", policyErr)
if policyErr != nil {
	var awsErr smithy.APIError
	if errors.As(policyErr, &awsErr) && awsErr.ErrorCode() == "NoSuchAccessPointPolicy" {
		ko.Spec.Policy = nil
	} else {
		return nil, policyErr
	}
} else {
	ko.Spec.Policy = policyResp.Policy
}

// The tagging APIs are not documented for S3 on Outposts access points, so
// their tags are left as declared and produce no delta.
if ko.Status.ACKResourceMetadata != nil && ko.Status.ACKResourceMetadata.ARN != nil &&
	tagsSupported(string(*ko.Status.ACKResourceMetadata.ARN)) {
	ko.Spec.Tags, err = rm.getTags(ctx, r.ko.Spec.AccountID, string(*ko.Status.ACKResourceMetadata.ARN))
	if err != nil {
		return nil, err
	}
}
