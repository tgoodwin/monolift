namespace MonoliftLean

/-- The deployment modes we distinguish before introducing a network model. -/
inductive PlacementMode
  | stateless
  | singleton
  | externalState
  deriving Repr, DecidableEq

/-- A small effect vocabulary for the first liftability model.

The location class is deliberately abstract. A Go SSA analysis can later map
parameter-derived stores, owned receiver stores, and external effects into this
vocabulary without depending on a Go representation.
-/
inductive Owner
  | caller
  | region
  | external
  deriving Repr, DecidableEq

inductive Event
  | read (owner : Owner)
  | write (owner : Owner) (value : Nat)
  | escape (owner : Owner)
  deriving Repr, DecidableEq

abbrev Trace := List Event

/-- A write is permitted only when its owner matches the selected placement.
    Caller-owned locations and aliases never cross this model's cut. -/
def safeEvent : PlacementMode → Event → Prop
  | _, .read _ => True
  | .stateless, .write _ _ => False
  | .singleton, .write .region _ => True
  | .externalState, .write .external _ => True
  | _, .write .caller _ => False
  | _, .write .external _ => False
  | _, .write .region _ => False
  | _, .escape _ => False

def safeTrace (mode : PlacementMode) : Trace → Prop
  | [] => True
  | event :: rest => safeEvent mode event ∧ safeTrace mode rest

structure Operation where
  body : Trace
  hasErrorChannel : Bool
  deriving Repr

/-- This is the pre-transport notion of liftability. It says nothing about
    codecs, retries, or network delivery. -/
def Liftable (mode : PlacementMode) (operation : Operation) : Prop :=
  operation.hasErrorChannel = true ∧ safeTrace mode operation.body

structure Store where
  caller : Nat
  region : Nat
  external : Nat
  deriving Repr, DecidableEq

def Store.apply (store : Store) : Event → Store
  | .read _ => store
  | .escape _ => store
  | .write .caller value => { store with caller := value }
  | .write .region value => { store with region := value }
  | .write .external value => { store with external := value }

def run : Trace → Store → Store
  | [], store => store
  | event :: rest, store => run rest (Store.apply store event)

/-- An admitted trace cannot change caller-owned state. This is the first
    frame property needed before introducing an extracted service. -/
theorem run_preserves_caller_of_safe (mode : PlacementMode) :
    ∀ (trace : Trace) (store : Store),
      safeTrace mode trace → (run trace store).caller = store.caller := by
  intro trace
  induction trace with
  | nil =>
      intro store _
      rfl
  | cons event rest ih =>
      intro store safe
      cases event with
      | read owner =>
          exact ih store safe.2
      | escape owner =>
          exact ih store safe.2
      | write owner value =>
          cases mode with
          | stateless =>
              simp only [safeTrace, safeEvent, false_and] at safe
          | singleton =>
              cases owner with
              | caller =>
                  simp only [safeTrace, safeEvent, false_and] at safe
              | region =>
                  exact ih _ safe.2
              | external =>
                  simp only [safeTrace, safeEvent, false_and] at safe
          | externalState =>
              cases owner with
              | caller =>
                  simp only [safeTrace, safeEvent, false_and] at safe
              | region =>
                  simp only [safeTrace, safeEvent, false_and] at safe
              | external =>
                  exact ih _ safe.2

-- The examples correspond to small Go fixtures. The trace is the semantic
-- summary that the Go analyzer will eventually produce.
def pureOperation : Operation :=
  { body := [], hasErrorChannel := true }

def setFirst : Operation :=
  { body := [.write .caller 1], hasErrorChannel := true }

def returnInputAlias : Operation :=
  { body := [.escape .caller], hasErrorChannel := true }

def ownedIncrement : Operation :=
  { body := [.write .region 1], hasErrorChannel := true }

def externalIncrement : Operation :=
  { body := [.write .external 1], hasErrorChannel := true }

def noErrorResult : Operation :=
  { body := [], hasErrorChannel := false }

theorem pureOperation_is_liftable : Liftable .stateless pureOperation := by
  simp [Liftable, pureOperation, safeTrace]

theorem setFirst_is_not_liftable : ¬ Liftable .stateless setFirst := by
  simp [Liftable, setFirst, safeTrace, safeEvent]

theorem returnInputAlias_is_not_liftable : ¬ Liftable .stateless returnInputAlias := by
  simp [Liftable, returnInputAlias, safeTrace, safeEvent]

theorem ownedIncrement_is_singleton_liftable : Liftable .singleton ownedIncrement := by
  simp [Liftable, ownedIncrement, safeTrace, safeEvent]

theorem ownedIncrement_is_not_stateless : ¬ Liftable .stateless ownedIncrement := by
  simp [Liftable, ownedIncrement, safeTrace, safeEvent]

theorem externalIncrement_is_externalState_liftable :
    Liftable .externalState externalIncrement := by
  simp [Liftable, externalIncrement, safeTrace, safeEvent]

theorem noErrorResult_is_not_liftable : ¬ Liftable .stateless noErrorResult := by
  simp [Liftable, noErrorResult, safeTrace]

theorem pureOperation_preserves_caller (store : Store) :
    (run pureOperation.body store).caller = store.caller := by
  exact run_preserves_caller_of_safe .stateless pureOperation.body store
    (by simp [safeTrace, pureOperation])

theorem ownedIncrement_preserves_caller (store : Store) :
    (run ownedIncrement.body store).caller = store.caller := by
  exact run_preserves_caller_of_safe .singleton ownedIncrement.body store
    (by simp [safeTrace, ownedIncrement, safeEvent])

end MonoliftLean
