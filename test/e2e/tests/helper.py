# Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License"). You may
# not use this file except in compliance with the License. A copy of the
# License is located at
#
#	 http://aws.amazon.com/apache2.0/
#
# or in the "license" file accompanying this file. This file is distributed
# on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either
# express or implied. See the License for the specific language governing
# permissions and limitations under the License.

"""Helper functions for S3Control e2e tests
"""

import logging

class S3ControlValidator:
    def __init__(self, s3control_client):
        self.s3control_client = s3control_client

    def get_access_point(self, account_id: str, name: str) -> dict:
        try:
            resp = self.s3control_client.get_access_point(
                AccountId=account_id,
                Name=name,
            )
            return resp

        except Exception as e:
            return None

    def access_point_exist(self, account_id: str, name: str) -> bool:
        return self.get_access_point(account_id, name) is not None

