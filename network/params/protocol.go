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
	// CumputeCapacity refers to the maximum computational effort supported per block by the
	// consensus protocol.
	CumputeCapacity uint64 = 1000

	// OracleReportInterval represents the time interval (in blocks) between
	// reports.
	OracleReportInterval uint64 = 900

	// OracleReportSubmissionPeriod represents the period of time available (in
	// blocks) for the report submission.
	OracleReportSubmissionPeriod uint64 = 15

	// StabilityFeeIncreasePercentage we will periodically increase the stability fee
	// by the “tolerably small amount” of 9% until it rises to its maximum value
	// (StabilityFeeMax).
	StabilityFeeIncreasePercentage uint64 = 9

	// StabilityFeeMaxPercentage represents the tx amount percentage that corresponds
	// to the max stability fee. According to the whitepaper, the maximum stability
	// fee is now 2% of the transaction amount.
	StabilityFeeMaxPercentage uint64 = 2

	// BlockTime (ms) refers to the maximum time that it takes to mine a block.
	BlockTime uint64 = 3000

	// WARNING: the following values (ms) must make sense give the block time.
	/*
		ProposeDuration        uint64 = 1000
		ProposeDeltaDuration   uint64 = 50
		PreVoteDuration        uint64 = 900
		PreVoteDeltaDuration   uint64 = 25
		PreCommitDuration      uint64 = 900
		PreCommitDeltaDuration uint64 = 50
	*/
)
