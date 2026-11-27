import GoLean.GoCore.Multi
import GoLean.NativeToIR
import GoLean.StrictJsonParse
import MonoliftLean.SingleAttempt

namespace MonoliftLean

open GoLean GoLean.GoCore

/-- The fixture only admits a nil Go error result, represented here by `Unit`. -/
abbrev NegateReply := Bool × Unit

def evalNegate (program : Program) (input : Bool) : Except String NegateReply := do
  match Machine.runProgramPoolOutM 1000 program "negate" #[.bool input] with
  | .error (stop, _) => throw s!"GoLean stopped: {repr stop}"
  | .ok result =>
    match result.values.toList with
    | [GoValue.bool value, GoValue.nil] =>
      if result.output.bytes.isEmpty then pure (value, ())
      else throw "negate produced unexpected output"
    | _ => throw "negate did not return (bool, nil)"

def identityCodec (α : Type) : Codec α α := ⟨id, id⟩

theorem negate_delivered_preserves_direct (program : Program) (input : Bool) :
    singleAttempt (identityCodec Bool) (identityCodec NegateReply)
        (evalNegate program) input .delivered =
      (evalNegate program input).map (fun result => ⟨.ok result, 1⟩) := by
  exact delivered_preserves_direct (identityCodec Bool) (identityCodec NegateReply)
    (evalNegate program) input (fun _ => rfl) (fun _ => rfl)

private def checkNegate (program : Program) (input : Bool) : Except String NegateReply := do
  let direct ← evalNegate program input
  let attempt := singleAttempt (identityCodec Bool) (identityCodec NegateReply)
    (evalNegate program) input
  match ← attempt .delivered with
  | ⟨.ok reply, 1⟩ =>
    if reply != direct then throw "delivered reply differs from direct evaluation"
  | report => throw s!"expected a reply and one execution, got {repr report}"
  let requestLost ← attempt .requestLost
  let replyLost ← attempt .replyLost
  match requestLost, replyLost with
  | ⟨.error .lost, 0⟩, ⟨.error .lost, 1⟩ => pure direct
  | _, _ => throw s!"unexpected loss reports: {repr requestLost}, {repr replyLost}"

end MonoliftLean

def main (args : List String) : IO Unit := do
  let [wire] := args | throw (IO.userError "usage: lean-experiment <wire.json>")
  let bytes ← IO.FS.readBinFile wire
  let program ← IO.ofExcept do
    let json ← GoLean.StrictJson.parseBytes bytes GoLean.StrictJson.wireNestingDepth
    GoLean.NativeToIR.decodeProgram json
  for input in [false, true] do
    match MonoliftLean.checkNegate program input with
    | .ok (value, ()) => IO.println s!"{input} {value} true"
    | .error err => throw (IO.userError s!"negate({input}): {err}")
