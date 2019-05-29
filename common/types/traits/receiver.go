// Copyright 2018 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package traits

import "github.com/google/cel-go/common/types/ref"

// Receiver interface for routing instance method calls within a value.
type Receiver interface {
	// ReceiveUnary applies a function on the current value.
	ReceiveUnary(function, overload string) ref.Val

	// ReceiveBinary applies a function and takes the current object as the left-hand side arg in
	// a binary operation against the rhs value.
	ReceiveBinary(function, overload string, rhs ref.Val) ref.Val

	// Receive accepts a function name, overload id, and arguments and returns a value.
	Receive(function, overload string, args []ref.Val) ref.Val
}
