# Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License"). You may
# not use this file except in compliance with the License. A copy of the
# License is located at
#
# 	 http://aws.amazon.com/apache2.0/
#
# or in the "license" file accompanying this file. This file is distributed
# on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either
# express or implied. See the License for the specific language governing
# permissions and limitations under the License.

"""Integration tests for the S3 Control Access Point API.
"""

import json
import pytest
import time
import logging

from acktest.resources import random_suffix_name
from acktest.k8s import resource as k8s
from acktest.aws.identity import get_account_id

from e2e import service_marker, CRD_GROUP, CRD_VERSION, load_s3control_resource
from e2e.replacement_values import REPLACEMENT_VALUES
from e2e.bootstrap_resources import get_bootstrap_resources
from e2e.tests.helper import S3ControlValidator

RESOURCE_PLURAL = "accesspoints"

CREATE_WAIT_AFTER_SECONDS = 10
UPDATE_WAIT_AFTER_SECONDS = 10
DELETE_WAIT_AFTER_SECONDS = 10

@pytest.fixture(scope="module")
def simple_access_point(s3control_client):

    resource_name = random_suffix_name("accesspoint", 24)

    account_id = get_account_id()
    replacements = REPLACEMENT_VALUES.copy()
    replacements["ACCESS_POINT_NAME"] = resource_name
    replacements["ACCOUNT_ID"] = account_id
    replacements["BUCKET_NAME"] = get_bootstrap_resources().Bucket.name

    resource_data = load_s3control_resource(
        "accesspoint",
        additional_replacements=replacements,
    )

    logging.debug(resource_data)

    # Create k8s resource
    ref = k8s.CustomResourceReference(
        CRD_GROUP, CRD_VERSION, RESOURCE_PLURAL,
        resource_name, namespace="default",
    )
    k8s.create_custom_resource(ref, resource_data)

    time.sleep(CREATE_WAIT_AFTER_SECONDS)
    cr = k8s.wait_resource_consumed_by_controller(ref)

    assert cr is not None
    assert k8s.get_resource_exists(ref)

    yield (ref, cr, resource_name)

    _, deleted = k8s.delete_custom_resource(
        ref,
        period_length=DELETE_WAIT_AFTER_SECONDS,
    )
    assert deleted

    time.sleep(DELETE_WAIT_AFTER_SECONDS)

    validator = S3ControlValidator(s3control_client)
    assert not validator.access_point_exist(account_id, resource_name)


def _make_policy(account_id: str, ap_name: str) -> str:
    """Return a simple S3 access point policy as a JSON string."""
    policy = {
        "Version": "2012-10-17",
        "Statement": [
            {
                "Effect": "Allow",
                "Principal": {"AWS": f"arn:aws:iam::{account_id}:root"},
                "Action": "s3:GetObject",
                "Resource": (
                    f"arn:aws:s3:us-east-1:{account_id}:accesspoint/{ap_name}/object/*"
                ),
            }
        ],
    }
    return json.dumps(policy)


@service_marker
@pytest.mark.canary
class TestAccessPoint:
    def test_create_delete(self, s3control_client, simple_access_point):
        (ref, _, access_point_name) = simple_access_point
        assert access_point_name is not None
        account_id = get_account_id()

        validator = S3ControlValidator(s3control_client)
        assert validator.access_point_exist(account_id, access_point_name)

    def test_create_with_policy(self, s3control_client):
        """TC-2: Create an access point with a policy and verify it is set in AWS."""
        resource_name = random_suffix_name("ap-policy", 24)
        account_id = get_account_id()
        policy_doc = _make_policy(account_id, resource_name)

        replacements = REPLACEMENT_VALUES.copy()
        replacements["ACCESS_POINT_NAME"] = resource_name
        replacements["ACCOUNT_ID"] = account_id
        replacements["BUCKET_NAME"] = get_bootstrap_resources().Bucket.name
        replacements["POLICY_DOCUMENT"] = policy_doc

        resource_data = load_s3control_resource(
            "accesspoint_with_policy",
            additional_replacements=replacements,
        )

        ref = k8s.CustomResourceReference(
            CRD_GROUP, CRD_VERSION, RESOURCE_PLURAL,
            resource_name, namespace="default",
        )

        try:
            k8s.create_custom_resource(ref, resource_data)
            time.sleep(CREATE_WAIT_AFTER_SECONDS)
            cr = k8s.wait_resource_consumed_by_controller(ref)

            assert cr is not None
            assert k8s.get_resource_exists(ref)

            validator = S3ControlValidator(s3control_client)
            assert validator.access_point_exist(account_id, resource_name)
            aws_policy = validator.get_access_point_policy(account_id, resource_name)
            assert aws_policy is not None, "Expected policy to be set in AWS"
        finally:
            k8s.delete_custom_resource(ref, period_length=DELETE_WAIT_AFTER_SECONDS)
            time.sleep(DELETE_WAIT_AFTER_SECONDS)

    def test_update_policy(self, s3control_client):
        """TC-3: Create access point without policy, then patch to add one."""
        resource_name = random_suffix_name("ap-upd-pol", 24)
        account_id = get_account_id()

        replacements = REPLACEMENT_VALUES.copy()
        replacements["ACCESS_POINT_NAME"] = resource_name
        replacements["ACCOUNT_ID"] = account_id
        replacements["BUCKET_NAME"] = get_bootstrap_resources().Bucket.name

        resource_data = load_s3control_resource(
            "accesspoint",
            additional_replacements=replacements,
        )

        ref = k8s.CustomResourceReference(
            CRD_GROUP, CRD_VERSION, RESOURCE_PLURAL,
            resource_name, namespace="default",
        )

        try:
            k8s.create_custom_resource(ref, resource_data)
            time.sleep(CREATE_WAIT_AFTER_SECONDS)
            cr = k8s.wait_resource_consumed_by_controller(ref)
            assert cr is not None

            # Verify no policy initially
            validator = S3ControlValidator(s3control_client)
            assert validator.get_access_point_policy(account_id, resource_name) is None

            # Patch with a policy
            policy_doc = _make_policy(account_id, resource_name)
            patch = {"spec": {"policy": policy_doc}}
            k8s.patch_custom_resource(ref, patch)
            time.sleep(UPDATE_WAIT_AFTER_SECONDS)

            aws_policy = validator.get_access_point_policy(account_id, resource_name)
            assert aws_policy is not None, "Expected policy to be set after update"
        finally:
            k8s.delete_custom_resource(ref, period_length=DELETE_WAIT_AFTER_SECONDS)
            time.sleep(DELETE_WAIT_AFTER_SECONDS)

    def test_delete_policy(self, s3control_client):
        """TC-4: Create access point with policy, then remove it via patch."""
        resource_name = random_suffix_name("ap-del-pol", 24)
        account_id = get_account_id()
        policy_doc = _make_policy(account_id, resource_name)

        replacements = REPLACEMENT_VALUES.copy()
        replacements["ACCESS_POINT_NAME"] = resource_name
        replacements["ACCOUNT_ID"] = account_id
        replacements["BUCKET_NAME"] = get_bootstrap_resources().Bucket.name
        replacements["POLICY_DOCUMENT"] = policy_doc

        resource_data = load_s3control_resource(
            "accesspoint_with_policy",
            additional_replacements=replacements,
        )

        ref = k8s.CustomResourceReference(
            CRD_GROUP, CRD_VERSION, RESOURCE_PLURAL,
            resource_name, namespace="default",
        )

        try:
            k8s.create_custom_resource(ref, resource_data)
            time.sleep(CREATE_WAIT_AFTER_SECONDS)
            cr = k8s.wait_resource_consumed_by_controller(ref)
            assert cr is not None

            validator = S3ControlValidator(s3control_client)
            assert validator.get_access_point_policy(account_id, resource_name) is not None

            # Remove policy by setting it to null
            patch = {"spec": {"policy": None}}
            k8s.patch_custom_resource(ref, patch)
            time.sleep(UPDATE_WAIT_AFTER_SECONDS)

            aws_policy = validator.get_access_point_policy(account_id, resource_name)
            assert aws_policy is None, "Expected policy to be removed after patch"
        finally:
            k8s.delete_custom_resource(ref, period_length=DELETE_WAIT_AFTER_SECONDS)
            time.sleep(DELETE_WAIT_AFTER_SECONDS)
