// Copyright © 2018 Kowala SEZC <info@kowala.tech>
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package params

const (
	// MaxTransactionsPerBlock refers to the maximum transactions supported
	// by the consensus protocol per block.
	MaxTransactionsPerBlock = 1000

	// OracleReportInterval represents the time interval (in blocks) between reports.
	OracleReportInterval = 900

	// OracleReportSubmissionPeriod represents the period of time available
	// (in blocks) for the report submission.
	OracleReportSubmissionPeriod = 15

	// StabilityFeeIncrease we will periodically increase the
	// stability fee by the “tolerably small amount” of 9% until it rises
	// to its maximum value (StabilityFeeMax).
	StabilityFeeIncrease = 9

	// StabilityFeeMax represents the maximum value
	// of the stability fee (2 % of the transaction amount)
	StabilityFeeMax = 2
)
