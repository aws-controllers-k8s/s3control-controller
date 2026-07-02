if desired.ko.Spec.Policy != nil && *desired.ko.Spec.Policy != "" {
	policyInput := &svcsdk.PutAccessPointPolicyInput{
		AccountId: desired.ko.Spec.AccountID,
		Name:      desired.ko.Spec.Name,
		Policy:    desired.ko.Spec.Policy,
	}
	_, err = rm.sdkapi.PutAccessPointPolicy(ctx, policyInput)
	if err != nil {
		return nil, err
	}
	ko.Spec.Policy = desired.ko.Spec.Policy
}
