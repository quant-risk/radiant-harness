# Skill: quantum-physics

> Schrödinger equation, Hilbert spaces, entanglement, gates,
> measurement, decoherence. Quantum mechanics is the rule;
> classical intuition is the limiting case.

## Decision tree

```
Quantum system
        │
        ▼
[Step 1] Identify subfield (QM / QI / QC / QChem / QOptics)
        │
        ▼
[Step 2] Specify Hilbert space + basis
        │
        ▼
[Step 3] Identify Hamiltonian
        │
        ▼
[Step 4] Prepare initial state
        │
        ▼
[Step 5] Evolve (unitary / Lindblad)
        │
        ▼
[Step 6] Measure (observables, POVM)
        │
        ▼
[Step 7] Interpret (Born rule, ensemble)
```

## Workflow

### Postulates of QM

1. **State**: system in state |ψ⟩ ∈ H (Hilbert space)
2. **Observables**: A → Â (Hermitian operator)
3. **Measurement**: probability P(a) = |⟨a|ψ⟩|² (Born rule)
4. **Evolution**: |ψ(t)⟩ = U(t)|ψ(0)⟩; iℏ ∂U/∂t = HU (Schrödinger)
5. **Composite systems**: H_total = H_A ⊗ H_B

### Schrödinger equation

**Time-dependent**:
```
iℏ ∂|ψ⟩/∂t = Ĥ|ψ⟩
```

**Time-independent**:
```
Ĥ|ψ⟩ = E|ψ⟩
```

**Common Hamiltonians**:
- Free particle: Ĥ = -ℏ²/(2m) ∇²
- Harmonic oscillator: Ĥ = p²/(2m) + ½ mω²x²
- Hydrogen atom: Ĥ = -ℏ²/(2m)∇² - e²/(4πε₀r)
- Two-level: Ĥ = ½ℏ(ω₀σ_z + Ωσ_x)

### Hilbert space

| System | Dim | Basis |
|--------|-----|-------|
| Spin-1/2 | 2 | { |↑⟩, |↓⟩ } |
| Qubit | 2 | { |0⟩, |1⟩ } |
| Spin-1 | 3 | { |-1⟩, |0⟩, |+1⟩ } |
| n qubits | 2ⁿ | tensor products |
| Harmonic osc | ∞ | { |n⟩, n=0,1,2,... } |

**Inner product**: ⟨φ|ψ⟩ = ∫ φ* ψ dx

**Norm**: ⟨ψ|ψ⟩ = 1 (normalisation)

### Entanglement

**Bell state**: |Φ⁺⟩ = (|00⟩ + |11⟩)/√2

**Detection**: 
- Reduced density matrix ρ_A = Tr_B |ψ⟩⟨ψ|
- ρ_A is mixed → entangled
- Measure: entanglement entropy S(ρ_A) = -Tr(ρ_A log ρ_A)

**Bell inequality violation**: distinguishes QM from local hidden
variable theories.

### Quantum gates

| Gate | Matrix | Effect |
|------|--------|--------|
| X (NOT) | [[0,1],[1,0]] | bit flip |
| Z | [[1,0],[0,-1]] | phase flip |
| H (Hadamard) | 1/√2 [[1,1],[1,-1]] | creates superposition |
| CNOT | 4×4 controlled flip | creates entanglement |
| T | diag(1, e^{iπ/4}) | T gate |
| Toffoli | 8×8 controlled-controlled NOT | universal with H, T |

Universal gate sets: {H, T, CNOT} (Clifford+T).

### Density matrices

Mixed state: ρ = Σ p_i |ψ_i⟩⟨ψ_i|

Properties:
- ρ ≥ 0 (positive semidefinite)
- Tr(ρ) = 1
- ρ = ρ† (Hermitian)

Pure state: ρ = |ψ⟩⟨ψ| (rank 1, Tr(ρ²) = 1)
Mixed state: Tr(ρ²) < 1

### Measurement

**Projective**: P̂ = Σ |a⟩⟨a|; eigenvalues = measurement outcomes

**POVM (Positive Operator-Valued Measure)**:
- { E_a } with E_a ≥ 0; Σ E_a = I
- Probability P(a) = Tr(ρ E_a)
- More general than projective

**Weak measurement**: partial information; doesn't collapse fully.

### Decoherence

Open quantum system (interacts with environment):

**Lindblad master equation**:
```
dρ/dt = -i/ℏ [Ĥ, ρ] + Σ (L_k ρ L_k† - ½{L_k†L_k, ρ})
```

L_k = Lindblad operators (collapse operators).

**Decoherence time** T₂: superposition decay time.

NISQ devices: T₂ ~ 100 μs (superconducting); T₁ ~ 1 ms.

### Bell inequality

CHSH: |S| ≤ 2 (local hidden variables)
QM prediction: |S| = 2√2 ≈ 2.83

Experiments violate classical bound; confirm QM.

## Examples

### Example 1: two-level system

```
State: |ψ⟩ = α|0⟩ + β|1⟩, |α|² + |β|² = 1
Measurement of Z:
  P(0) = |α|²
  P(1) = |β|²
Hadamard then Z:
  H|ψ⟩ = (α + β)/√2 |0⟩ + (α - β)/√2 |1⟩
  P(0) = |α + β|² / 2
```

### Example 2: Bell state

```
|Φ⁺⟩ = (|00⟩ + |11⟩)/√2
Measure both qubits in Z basis:
  P(00) = 1/2; P(11) = 1/2; P(01) = P(10) = 0
  Correlation: ⟨Z₁Z₂⟩ = +1 (perfect)
Reduced density matrix:
  ρ_A = Tr_B |Φ⁺⟩⟨Φ⁺| = ½ I (maximally mixed)
Entanglement entropy = ln 2 (max for 2 qubits)
```

### Example 3: Rabi oscillation

```
Two-level atom in resonant field
Population: P_1(t) = sin²(Ωt/2)
Period: T = 2π/Ω
Limiting:
  - Ω → 0: P_1 → 0 (no driving)
  - t = π/(2Ω): P_1 = 1 (full inversion; π pulse)
  - t = π/Ω: P_1 = 0 (return to ground; 2π pulse)
```

## Anti-patterns

### ❌ Classical intuition in quantum regime

Position AND momentum simultaneously. Heisenberg: Δx Δp ≥ ℏ/2.

### ❌ Ignoring decoherence

Superposition lost in real systems on T₂ timescale.

### ❌ No measurement protocol

"Observation" ambiguous. Specify observable + basis.

### ❌ Norm drift in numerics

Unitarity lost in discretisation; use symplectic / unitary-preserving
methods.

### ❌ Classical analogy overreach

"Don't think of electron as a particle" — it's neither particle
nor wave; it's a quantum object.

## Failure modes

| Failure | Recovery |
|---------|----------|
| Decoherence dominates | Error correction; shorter circuits |
| Measurement collapses too soon | Weak measurement; deferred measurement |
| Entanglement hard to verify | Bell inequality; tomographic reconstruction |
| Simulation intractable | Tensor networks; truncation; variational methods |
| Numerical instability | Symplectic / unitary integrator |

## Related skills

| Skill | When to chain |
|-------|---------------|
| `/physics` | Classical limit |
| `/quantum-ml` | Quantum ML algorithms |
| `/chemistry` | Quantum chemistry |
| `/bayesian` | Quantum state tomography; prior elicitation |

## Citations

- Nielsen & Chuang, "Quantum Computation and Quantum Information"
- Sakurai, "Modern Quantum Mechanics"
- Cohen-Tannoudji, "Quantum Mechanics"
- Preskill, "Lecture Notes on Quantum Computation" (open)