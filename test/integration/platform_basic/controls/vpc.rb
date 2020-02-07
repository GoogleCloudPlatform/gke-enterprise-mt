# Copyright 2020 Google LLC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     https://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

vpc_shared_net_prod_name = attribute('vpc_shared_net_prod_name')
subnets = attribute('vpc_shared_net_subnets')

control "VPC" do
  title "VPC configuration"

  it "should have only one subnet" do
    expect(subnets.length).to eq 1
  end

  if "should be a prod subnet" do
    expect(subnets[0].subnet_name.to eq "prod-subnet"
  end
end
