namespace MonoliftLean

structure Codec (Value Wire : Type) where
  encode : Value → Wire
  decode : Wire → Value

def Codec.RoundTrips (codec : Codec Value Wire) : Prop :=
  ∀ value, codec.decode (codec.encode value) = value

inductive Delivery where
  | delivered
  | requestLost
  | replyLost

inductive CommunicationError where
  | lost
  deriving Repr

/-- Only `caller` is visible to the caller. `executions` is model bookkeeping. -/
structure AttemptReport (Reply : Type) where
  caller : Except CommunicationError Reply
  executions : Nat
  deriving Repr

/-- An evaluator refusal is separate from a communication error or a Go reply.
    Reply loss discards the result of one evaluation; neither loss retries. -/
def singleAttempt (request : Codec Arg RequestWire) (reply : Codec Reply ReplyWire)
    (callee : Arg → Except EvalError Reply) (input : Arg) :
    Delivery → Except EvalError (AttemptReport Reply)
  | .requestLost => .ok ⟨.error .lost, 0⟩
  | .replyLost => (callee (request.decode (request.encode input))).map fun _ =>
      ⟨.error .lost, 1⟩
  | .delivered => (callee (request.decode (request.encode input))).map fun result =>
      ⟨.ok (reply.decode (reply.encode result)), 1⟩

theorem delivered_preserves_direct (request : Codec Arg RequestWire)
    (reply : Codec Reply ReplyWire) (callee : Arg → Except EvalError Reply) (input : Arg)
    (requestRoundTrips : request.RoundTrips) (replyRoundTrips : reply.RoundTrips) :
    singleAttempt request reply callee input .delivered =
      (callee input).map (fun result => ⟨.ok result, 1⟩) := by
  simp only [singleAttempt, requestRoundTrips input]
  cases callee input with
  | error _ => rfl
  | ok result =>
    simp only [Except.map, replyRoundTrips result]

theorem lost_request_and_reply (request : Codec Arg RequestWire)
    (reply : Codec Reply ReplyWire) (callee : Arg → Except EvalError Reply) (input : Arg)
    (requestRoundTrips : request.RoundTrips) {result : Reply} (completed : callee input = .ok result) :
    singleAttempt request reply callee input .requestLost = .ok ⟨.error .lost, 0⟩ ∧
    singleAttempt request reply callee input .replyLost = .ok ⟨.error .lost, 1⟩ := by
  simp only [singleAttempt, requestRoundTrips input, completed, Except.map, and_self]

end MonoliftLean
