# Skill: quantum-ml

> VQE, QAOA, VQC, quantum kernels, NISQ constraints, error
> mitigation. Quantum advantage is rare; classical baseline
> is non-negotiable.

## Decision tree

```
Quantum ML question
        │
        ▼
[Step 1] Why quantum? (problem structure favours it?)
        │
        ▼
[Step 2] Choose algorithm (VQE / QAOA / VQC / kernel)
        │
        ▼
[Step 3] Design ansatz (depth, parameters)
        │
        ▼
[Step 4] Backend (simulator / hardware)
        │
        ▼
[Step 5] Train (hybrid optimisation)
        │
        ▼
[Step 6] Error mitigation
        │
        ▼
[Step 7] Compare vs classical baseline
```

## Workflow

### When to use quantum

Quantum has potential advantage for:
- **Quantum simulation**: chemistry, materials, condensed matter
  (Feynman's original motivation)
- **Samplers**: Boltzmann machines, certain combinatorial
- **Kernel methods**: quantum kernel expressivity (specific structure)
- **Optimisation**: QAOA for specific NP-hard problems

Quantum does NOT have advantage (provably or empirically) for:
- General classical ML (SVMs, neural nets on classical data)
- Random optimisation
- Generic search (without structure)

**Always run classical baseline first.**

### Variational Quantum Eigensolver (VQE)

Finds ground state energy of a Hamiltonian:

```
H|ψ⟩ = E|ψ⟩ (lowest eigenvalue)
|ψ(θ)⟩ = U(θ)|0⟩
E(θ) = ⟨ψ(θ)|H|ψ(θ)⟩
```

**Algorithm**:
1. Prepare ansatz state |ψ(θ)⟩
2. Measure expectation ⟨H⟩
3. Classical optimiser updates θ
4. Repeat

**Ansätze**: hardware-efficient (layers of R_y, R_z, CNOT);
UCCSD (chemistry-inspired); problem-inspired.

### QAOA (Quantum Approximate Optimisation Algorithm)

For combinatorial optimisation (MaxCut, TSP, portfolio):

```
|ψ(γ, β)⟩ = U_M(β_p) U_C(γ_p) ... U_M(β_1) U_C(γ_1) |s⟩
```

|s⟩ = uniform superposition; p = circuit depth.

**Cost operator** U_C encodes objective; **Mixer** U_M explores.

Approximation ratio depends on p.

### Variational Quantum Classifier (VQC)

Hybrid quantum-classical classifier:

```
|x⟩ → feature map (encoding) → variational layers → measure → classical post-process → y
```

**Feature maps**: angle encoding, amplitude encoding, IQP-style.

**Train**: parameter-shift rule for gradient; classical optimiser.

### Quantum kernels

Quantum computer computes kernel K(x, x') = |⟨φ(x)|φ(x')⟩|²

Then plug into classical kernel methods (SVM, GP).

**Risk**: kernel may not be classically hard to compute; verify
empirical advantage.

### NISQ constraints

| Constraint | Typical value |
|-----------|---------------|
| Qubits | 50-1000 (current) |
| Circuit depth | <100 before decoherence |
| Connectivity | Limited (nearest-neighbour typical) |
| Gate fidelity | 99-99.9% per gate |
| Measurement | Slow; expensive |
| T₂ | 100 μs (superconducting) |

Consequence: shallow circuits, many parameters via re-uploading
or measurement feedback.

### Error mitigation

| Technique | Effect |
|-----------|--------|
| **Zero-noise extrapolation** | Run at multiple noise levels; extrapolate to zero |
| **Probabilistic error cancellation** | Invert noise channel; sample from distribution |
| **Readout error correction** | Calibration matrix; invert |
| **Dynamical decoupling** | Pulse sequences to suppress decoherence |
| **Measurement post-selection** | Discard bad runs |

### Hybrid quantum-classical pipelines

Most useful NISQ pattern:

```
Classical preprocessing → quantum kernel / circuit → classical post-processing
```

Example: feature selection classical → quantum kernel SVM → classical threshold.

## Examples

### Example 1: VQE for H2

```
Molecule: H2, bond length 0.74 Å
Hamiltonian: H = g₀ I + g₁ Z₀ + g₂ Z₁ + g₃ Z₀Z₁ + g₄ X₀X₁ + g₅ Y₀Y₁
Ansatz: hardware-efficient, 2 qubits, depth 2
Parameters: 4 angles θ_1..θ_4
Optimiser: COBYLA, max 200 iterations
Result: E ≈ -1.137 Ha (matches FCI within chemical accuracy)
Compare: classical CI = -1.137 Ha (parity)
```

### Example 2: QAOA for MaxCut

```
Graph: 4 nodes, 5 edges (square + diagonal)
p = 2 (2 layers)
Parameters: γ_1, γ_2, β_1, β_2 (4 total)
Optimiser: COBYLA
Approximation ratio: 0.95 (vs 0.85 random)
```

### Example 3: VQC on Iris (2-class)

```
Data: 2 features (sepal length, width); 100 samples
Feature map: angle encoding (2 qubits)
Variational: 3 layers, 12 parameters
Train: 50 epochs, Adam
Test accuracy: 93%
Classical baseline (SVM): 95%
Quantum: no advantage here; classical wins
```

## Anti-patterns

### ❌ Quantum without classical baseline

No advantage shown. Always run classical first.

### ❌ Deep circuits on NISQ

Decoherence destroys signal. Stay shallow (< 100 gates).

### ❌ No error mitigation

Noise dominates. Always mitigate.

### ❌ Pretending quantum solves NP-hard

Grover gives sqrt(N), not polynomial. Quantum doesn't solve NP.

### ❌ Encoding classical data poorly

Encoding overhead kills quantum advantage; use efficient maps.

### ❌ Reproducibility broken

Different runs → different results (noise); report statistics.

## Failure modes

| Failure | Recovery |
|---------|----------|
| Barren plateau | Better ansatz; local cost; layer-wise training |
| Noise dominates | Error mitigation; shorter circuit; better hardware |
| Classical beats quantum | OK; report honestly; classical may be correct choice |
| Optimisation stuck | Multiple initialisations; classical preprocessing |
| Connectivity mismatch | SWAP networks; transpilation |
| Reproducibility broken | Statistics over runs; error mitigation |

## Related skills

| Skill | When to chain |
|-------|---------------|
| `/quantum-physics` | Underlying QM formalism |
| `/ml` | Classical ML; baseline |
| `/physics` | Classical optimisation context |
| `/chemistry` | Quantum chemistry; VQE applications |

## Tools

| Framework | Purpose |
|-----------|---------|
| **Qiskit** (IBM) | Quantum SDK + runtime |
| **Cirq** (Google) | Quantum circuits + simulators |
| **PennyLane** | Quantum ML; differentiable |
| **PyQuil** (Rigetti) | Quantum instruction |
| **Q#** (Microsoft) | Quantum language |
| **cuQuantum** (NVIDIA) | GPU-accelerated simulation |

## Citations

- Cerezo et al., "Variational Quantum Algorithms", Nature Reviews Physics 2021
- Biamonte et al., "Quantum Machine Learning", Nature 2017
- Schuld & Petruccione, "Supervised Learning with Quantum Computers"
- Preskill, "Quantum Computing in the NISQ era and beyond", 2018