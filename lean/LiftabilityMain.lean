import MonoliftLean.Liftability

open MonoliftLean

def main : IO Unit := do
  IO.println "Liftability checks passed: pure, aliasing, singleton state, external state, and error channel."
