if ko.Status.ACKResourceMetadata != nil && ko.Status.ACKResourceMetadata.ARN != nil {
	policyInput := &svcsdk.GetAccessPointPolicyInput{
		AccountId: r.ko.Spec.AccountID,
		Name:      r.ko.Spec.Name,
	}
	policyResp, policyErr := rm.sdkapi.GetAccessPointPolicy(ctx, policyInput)
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
}
