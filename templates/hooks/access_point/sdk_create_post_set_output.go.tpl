if ko.Spec.Policy != nil {
	ackcondition.SetSynced(&resource{ko}, corev1.ConditionFalse, nil, nil)
}
