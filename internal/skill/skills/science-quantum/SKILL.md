# Skill: science-quantum

## Overview

Science & Quantum covers computational biology, cheminformatics, physics simulation, quantum computing, quantum ML, Bayesian inference, and statistical modeling. This skill merges domain knowledge from 7 specialized areas with implementation patterns from Qiskit, PennyLane, BioPython, RDKit, SciPy, PyMC, and NumPyro.

**When to use**: Scientific computing, quantum algorithm design, molecular analysis, bioinformatics pipelines, or any domain requiring scientific/quantum expertise.


> Quantum computing (Qiskit, PennyLane), computational biology (BioPython),
> cheminformatics (RDKit), scientific computing (SciPy), physics simulation,
> Bayesian inference. Quantum advantage is rare; classical baselines are
> non-negotiable. Biology without controls is anecdote.

---

## Decision Tree

```
Science / Quantum problem
        │
        ▼
[Step 1] Identify domain
   ├── Quantum Computing → Step 2A
   ├── Computational Biology → Step 2B
   ├── Cheminformatics → Step 2C
   ├── Scientific Computing → Step 2D
   ├── Physics Simulation → Step 2E
   └── Bayesian Inference → Step 2F
        │
        ▼
[Step 2] Choose framework + method (see domain sections)
        │
        ▼
[Step 3] Implement pipeline
        │
        ▼
[Step 4] Validate vs experiment / classical baseline
        │
        ▼
[Step 5] Report with uncertainty + reproducibility
```

---

## Part I: Quantum Computing

### Frameworks

| Framework | Vendor | Strength |
|-----------|--------|----------|
| **Qiskit** | IBM | Hardware integration, transpiler (Rust-backed), runtime |
| **PennyLane** | Xanadu | Differentiable QC, ML integration, quantum chemistry |
| **Cirq** | Google | NISQ circuits, simulators |
| **cuQuantum** | NVIDIA | GPU-accelerated simulation |

### Qiskit Architecture (v1.x)

**Key insight**: Qiskit has a Rust backend (`_accelerate`) for performance-critical
paths — transpiler passes, DAG circuit operations, Pauli algebra, unitary synthesis.

**Module map**:
```
qiskit/
├── circuit/          # QuantumCircuit, Gate, Instruction, Parameter
│   ├── library/      # Standard gates, ansatz circuits
│   ├── controlflow/  # IfElseOp, ForLoopOp, WhileLoopOp, SwitchCaseOp
│   └── classical/    # Real-time classical expressions (Var, Expr, Type)
├── transpiler/       # PassManager, DAGCircuit, 100+ passes
│   └── preset_passmanagers/  # StagedPassManager (6 stages)
├── primitives/       # EstimatorV2, SamplerV2 (Statevector + Backend variants)
├── providers/        # Backend, Target, fake providers
├── quantum_info/     # Operator, Pauli, SparsePauliOp, Statevector, DensityMatrix, Clifford
├── synthesis/        # Circuit synthesis: euler, two-qubit, clifford, evolution, QFT
├── dagcircuit/       # DAG intermediate representation
├── compiler/         # transpile() top-level
├── qasm2/ qasm3/     # OpenQASM interchange
├── qpy/              # Binary serialization
└── visualization/    # Circuit drawing, DAG, timeline
```

**Core Abstractions**:

1. **QuantumCircuit**: Tree-based circuit. Contains qubits, clbits, parameters,
   and a sequence of `CircuitInstruction(operation, qubits, clbits)`.
   Abstract (virtual qubits) or physical (hardware qubits, ISA-compliant).

2. **DAGCircuit**: Graph-based IR for transpiler. Nodes = operations, edges =
   data dependencies. Enables graph algorithms for optimization.

3. **PassManager / StagedPassManager**: Pipeline of AnalysisPass (read-only) and
   TransformationPass (mutate DAG). Six stages:
   - `init`: unroll 3+ qubit ops, cancel inverses, elide permutations
   - `layout`: map virtual → hardware qubits (VF2Layout, SabreLayout, DenseLayout, TrivialLayout)
   - `routing`: insert SWAPs for connectivity (SabreSwap, StochasticSwap)
   - `translation`: convert to target basis gates (BasisTranslator, UnitarySynthesis)
   - `optimization`: hardware-aware peephole (ConsolidateBlocks, CommutativeCancellation)
   - `scheduling`: insert Delays, dynamical decoupling

4. **Primitives (V2)**: EstimatorV2 estimates ⟨ψ|H|ψ⟩; SamplerV2 samples circuit
   outputs. Both use PUBs (Primitive Unified Blocs) for vectorized input.
   StatevectorEstimator/Sampler for simulation; BackendEstimatorV2/Sampler for hardware.

5. **Target**: Describes hardware — available gates, connectivity, durations, error rates.

**Hidden Gems (Qiskit)**:
- `_accelerate` Rust backend: Sabre routing, euler decomposition, commutation analysis — 10-100x faster than Python.
- `EquivalenceLibrary`: database of gate decompositions for BasisTranslator.
- `qpy`: binary serialization for circuits — faster than QASM, preserves parameters.
- `SparseObservable`: efficient Pauli observable representation (Rust-backed).
- Clifford+T synthesis pipeline for fault-tolerant compilation.
- `generate_preset_pass_manager` with `seed_transpiler` for reproducibility.

**Anti-Patterns (Qiskit)**:
- ❌ Using `transpile()` without specifying backend/target — no hardware-aware optimization.
- ❌ Deep circuits on NISQ without error mitigation — decoherence destroys signal.
- ❌ Ignoring transpiler seed — non-reproducible results.
- ❌ Using V1 primitives — deprecated; use V2 with PUBs.

### PennyLane Architecture

**Key insight**: PennyLane is differentiable quantum computing — every operation
supports automatic differentiation via parameter-shift, backprop, or adjoint.
Tight integration with autograd, JAX, TensorFlow, PyTorch.

**Module map**:
```
pennylane/
├── workflow/        # QNode, execute(), construct_tape, set_shots
├── devices/         # DefaultQubit, DefaultMixed, DefaultClifford, DefaultTensor
├── ops/             # Operator classes: qubit/, qutrit/, op_math/, mid_measure/
├── core/operator/   # Base Operator class
├── measurements/    # expval, probs, sample, counts, state, var, vn_entropy
├── transforms/      # @transform decorator, circuit optimization, ZX calculus
├── gradients/       # parameter-shift, adjoint, backprop, hadamard, SPSA
├── templates/       # embeddings/, layers/, subroutines/, state_preparations/
├── optimize/        # GradientDescent, Adam, QNG, Rotosolve
├── qchem/           # Quantum chemistry: molecular Hamiltonians, VQE, tapering
├── qaoa/            # QAOA cost/mixer layers
├── fermi/           # Fermionic operators (FermiWord, FermiSentence, Jordan-Wigner)
├── bose/            # Bosonic operators and mappings
├── pauli/           # Pauli algebra, grouping, decomposition
├── noise/           # Noise models, ZNE error mitigation
├── kernels/         # Quantum kernels
├── pulse/           # Pulse-level control
├── decomposition/   # Gate decomposition registry
├── capture/         # Program capture for JIT compilation
├── io/              # Import/export: from_qiskit, from_qasm, to_openqasm
├── shadows/         # Classical shadows protocol
├── qcut/            # circuit cutting
└── resource/        # Resource estimation
```

**Core Abstractions**:

1. **Operator**: Base class for all quantum operations. Has `matrix()`, `eigvals()`,
   `decomposition()`, `generator()`. Supports `adjoint()`, `ctrl()`, `pow()`.
   Parameters are differentiable tensors.

2. **QNode**: Quantum function bound to a device. Supports interfaces (autograd,
   JAX, TF, Torch). `diff_method` controls gradient computation.

3. **Device**: Abstract quantum device. `DefaultQubit` for pure state, `DefaultMixed`
   for density matrix. Next-gen: `Device.preprocess()` → transform program.

4. **Tape (QuantumTape)**: Recordable quantum circuit via QueuingManager.
   Enables batched execution and transforms.

5. **Transforms**: Circuit-to-circuit. `@transform` decorator. Includes: compile,
   decompose, defer_measurements, batch_params, split_non_commuting.

**Hidden Gems (PennyLane)**:
- `decomposition` module: `register_resources`, `add_decomps` — declarative gate decomposition with resource tracking.
- `capture`: program capture with autograph for JIT compilation.
- `noise`: `NoiseModel`, `mitigate_with_zne`, `fold_global` — built-in error mitigation.
- `qchem`: molecular Hamiltonian construction, fermionic-to-qubit mappings, tapering (symmetry reduction).
- `qcut`: circuit cutting — run subcircuits independently for large circuits.
- `shadows`: classical shadow protocol for efficient state characterization.
- `pulse`: pulse-level control for hardware-aware optimization.
- `fourier`: Fourier analysis of quantum models (expressivity analysis).
- `labs/`: experimental — DLA (dynamical Lie algebra), Trotter error bounds.

**Anti-Patterns (PennyLane)**:
- ❌ Mixing frameworks (JAX device with Torch interface) — gradient errors.
- ❌ Not specifying `diff_method` — may default to inefficient method.
- ❌ Ignoring shot noise in hardware runs — report statistics.

### When to Use Quantum

| Problem | Quantum potential | Framework |
|---------|-------------------|-----------|
| Quantum chemistry (VQE) | High | Qiskit, PennyLane |
| Combinatorial optimization (QAOA) | Medium | Qiskit, PennyLane |
| Quantum kernels | Medium | PennyLane |
| General classical ML | None | — |
| Generic search | None (Grover: √N only) | — |

**Always run classical baseline first.**

### NISQ Constraints

| Constraint | Typical value |
|-----------|---------------|
| Qubits | 50-1000 |
| Circuit depth | <100 before decoherence |
| Gate fidelity | 99-99.9% |
| T₂ | 100 μs (superconducting) |

### Error Mitigation

| Technique | Effect |
|-----------|--------|
| Zero-noise extrapolation | Run at multiple noise levels; extrapolate to zero |
| Probabilistic error cancellation | Invert noise channel |
| Readout error correction | Calibration matrix inversion |
| Dynamical decoupling | Pulse sequences suppress decoherence |
| Measurement post-selection | Discard bad runs |

---

## Part II: Computational Biology (BioPython)

**Key insight**: BioPython handles sequences, alignments, phylogenetics, structures,
and database access. The `Seq` module is foundational — everything builds on it.

**Module map**:
```
Bio/
├── Seq.py / SeqRecord.py / SeqFeature.py   # Core sequence objects
├── SeqIO/              # FASTA, GenBank, FASTQ I/O
├── Align/              # MultipleSequenceAlignment, PairwiseAligner
│   └── substitution_matrices/  # BLOSUM62, PAM250
├── SearchIO/           # BLAST, HMMER, Exonerate result parsing
├── Phylo/              # Phylogenetic trees (Newick, Nexus)
│   └── PAML/           # Codeml, baseml wrappers
├── PDB/                # Protein structures (PDB, mmCIF, MMTF)
├── Blast/              # BLAST wrappers
├── Entrez/             # NCBI database access
├── KEGG/               # KEGG pathway database
├── UniProt/            # UniProt protein database
├── HMM/                # Hidden Markov Models
├── motifs/             # Sequence motifs (JASPAR)
├── Restriction/        # Restriction enzymes
├── Cluster/            # Gene expression clustering
├── Pathway/            # Biological pathways
├── Graphics/           # GenomeDiagram visualization
└── Data/               # Codon tables, IUPAC data
```

**Core Abstractions**:

1. **Seq**: Immutable sequence. Supports slicing, complement, reverse_complement,
   transcribe, translate. Backed by `SequenceDataAbstractBaseClass` for lazy loading.

2. **SeqRecord**: Annotated sequence. Contains Seq + id + name + description +
   features (SeqFeature list) + annotations + letter_annotations (per-residue).

3. **MultipleSequenceAlignment**: Aligned sequences. Column operations, substitution
   matrices, consensus calculation.

4. **PairwiseAligner**: Smith-Waterman, Needleman-Wunsch with scoring matrices.

**Genomics Pipeline**:
```
Raw reads (FASTQ) → QC → Trim → Align → Sort/Dedup → Variant call/Quantify
→ Differential expression → Pathway enrichment
```

**Hidden Gems (BioPython)**:
- `SequenceDataAbstractBaseClass`: lazy sequence loading — only parse requested regions.
- `SearchIO`: unified interface for BLAST, HMMER, Exonerate, Infernal results.
- `motifs`: JASPAR motif parsing, PSSM construction, sequence scanning.
- `Phylo.PAML`: wrappers for codeml/baseml (molecular evolution).
- `HMM`: basic Hidden Markov Model implementation.

**Anti-Patterns (BioPython)**:
- ❌ Loading entire genome into memory — use lazy parsing.
- ❌ Using `pairwise2` (deprecated) — use `Align.PairwiseAligner`.
- ❌ Not setting Entrez email — NCBI blocks anonymous queries.
- ❌ Ignoring alphabet deprecation — removed in v1.78+.

---

## Part III: Cheminformatics (RDKit)

**Key insight**: RDKit is C++ with Python bindings. Core molecule representation
(`rdchem.Mol`) is a C++ object — Python is a wrapper. Performance-critical
operations (substructure search, fingerprinting, descriptors) run in C++.

**Module map**:
```
rdkit/
├── Chem/
│   ├── rdchem.py          # Mol, Atom, Bond, EditableMol (C++ bindings)
│   ├── rdmolfiles.py      # SMILES/SD/Mol I/O
│   ├── rdmolops.py        # Molecular operations (AddHs, fingerprints)
│   ├── rdMolDescriptors.py # 200+ molecular descriptors
│   ├── AllChem.py         # 3D conformer generation, force fields
│   ├── Fingerprints/      # Morgan, MACCS, RDKit, Atom Pair fingerprints
│   ├── AtomPairs/         # Atom pair descriptors
│   ├── Pharm2D/ Pharm3D/  # Pharmacophore fingerprints
│   ├── Scaffolds/         # Murcko scaffold decomposition
│   ├── MolStandardize/    # Salt removal, tautomer enumeration
│   ├── Subshape/          # Molecular shape comparison
│   ├── Fraggle/           # Fragment-based similarity
│   ├── fmcs/              # Maximum Common Substructure
│   ├── Features/          # Pharmacophore feature definitions
│   ├── EState/            # E-state descriptors
│   ├── Suppliers/         # SD/SMILES suppliers (lazy iteration)
│   └── SimpleEnum/        # Reaction enumeration
├── DataStructs/           # Bit vectors for fingerprints
├── DataManip/             # Metric functions (Tanimoto, Dice, Cosine)
├── ML/                    # ML utilities (clustering, descriptors, scoring)
├── DistanceGeometry/      # 3D conformer generation
├── ForceField/            # UFF/MMFF force fields
└── Geometry/              # Point, plane, shape
```

**Core Abstractions**:

1. **Mol**: Central molecule (C++ backed). From SMILES, SDF, or SMARTS. Contains
   atoms, bonds, conformers, properties.

2. **Fingerprints**: Bit vectors encoding molecular features.
   - Morgan/ECFP: circular (atom environments)
   - MACCS: 166 structural keys
   - RDKit: path-based
   - Atom pairs: atom pair descriptors

3. **Descriptors**: 200+ (MW, LogP, TPSA, HBA, HBD, etc.)

4. **Reactions**: SMARTS-based reaction templates for retrosynthesis/forward prediction.

5. **Substructure search**: SMARTS pattern matching via `mol.HasSubstructMatch(pattern)`.

**Hidden Gems (RDKit)**:
- `MolStandardize`: salt removal, tautomer enumeration, canonicalization.
- `Scaffolds`: Murcko scaffold decomposition for scaffold-based analysis.
- `fmcs`: Maximum Common Substructure — largest shared substructure.
- `Pharm3D`: 3D pharmacophore fingerprints for shape-based similarity.
- `_GetRDKitObjIterator`: custom iterator avoiding boost::python exception overhead.

**Anti-Patterns (RDKit)**:
- ❌ Ignoring sanitization — check `mol is not None` after `MolFromSmiles`.
- ❌ Not removing Hs before substructure search — may miss matches.
- ❌ Default fingerprints without tuning radius/size — poor discrimination.
- ❌ Ignoring stereochemistry — SMILES parsing may lose it.

---

## Part IV: Scientific Computing (SciPy)

**Key insight**: SciPy wraps optimized C/Fortran code. The `optimize`, `stats`,
`signal`, `integrate`, `spatial`, and `linalg` modules are the workhorses.

**Module map**:
```
scipy/
├── optimize/           # minimize(), root(), curve_fit(), least_squares(), linprog
├── stats/              # 100+ distributions, statistical tests, KDE, QMC
├── signal/             # Filtering, peak detection, spectral analysis
├── integrate/          # quad(), solve_ivp() (ODE solvers: RK45, BDF, LSODA)
├── interpolate/        # CubicSpline, BSpline, RBF
├── linalg/             # LAPACK wrappers (eig, svd, lu, cholesky)
├── spatial/            # KDTree, Delaunay, Voronoi, ConvexHull, Rotation
├── special/            # Bessel, gamma, erf, etc.
├── sparse/             # CSR, CSC, COO + csgraph (Dijkstra, BFS, connected components)
├── cluster/            # hierarchy, vector quantization
├── fft/                # FFT (duccfft backend)
├── constants/          # Physical constants
├── ndimage/            # N-dimensional image processing
└── io/                 # MATLAB, ARFF, Matrix Market
```

### scipy.optimize

**Local**: `minimize(fun, x0, method='L-BFGS-B')` — Nelder-Mead, Powell, CG, BFGS,
L-BFGS-B, COBYLA, SLSQP, trust-constr. **Scalar**: `minimize_scalar` (Brent, Bounded).
**Global**: `differential_evolution`, `shgo`, `dual_annealing`. **Root**: `root` (hybr),
`root_scalar` (brentq). **Least squares**: `least_squares`, `curve_fit`.

### scipy.stats

**Distributions**: norm, t, chi2, f, beta, gamma, expon, lognorm, poisson, binom, etc.
Each has `.pdf()`, `.cdf()`, `.rvs()`, `.fit()`, `.interval()`.

**Tests**: ttest_ind, mannwhitneyu, chi2_contingency, f_oneway, kstest, shapiro.
**Multiple testing**: `multipletests(p_values, method='fdr_bh')`.

### scipy.integrate

`quad()` for single integrals, `dblquad()` for double. `solve_ivp()` for ODEs
with methods RK23, RK45, DOP853, Radau, BDF, LSODA. Use `dense_output=True`
for continuous solutions.

### scipy.signal

Butterworth/Chebyshev filter design, `filtfilt` for zero-phase filtering,
`find_peaks` for peak detection, `welch` for power spectral density.

### scipy.spatial

`KDTree` for nearest-neighbor search, `Delaunay` for triangulation,
`ConvexHull` for convex hulls, `cdist` for distance matrices,
`Rotation` for rotation representations (Euler, quaternion, rotation matrix).

**Hidden Gems (SciPy)**:
- `stats.multipletests`: Bonferroni, BH, BY, Holm correction.
- `signal.savgol_filter`: Savitzky-Golay smoothing — better than moving average.
- `spatial.KDTree.query_ball_point`: all points within radius.
- `sparse.csgraph`: graph algorithms on sparse matrices.
- `stats.qmc`: quasi-Monte Carlo (Sobol, Halton) — better than random sampling.
- `differentiate`: new module for numerical differentiation.

**Anti-Patterns (SciPy)**:
- ❌ Not specifying `method` in `minimize()` — defaults to BFGS.
- ❌ Using `interp1d` for large data — use `CubicSpline` or `BSpline`.
- ❌ Ignoring convergence warnings — result may be unreliable.
- ❌ t-test without checking assumptions — use non-parametric if violated.

---

## Part V: Physics Simulation

### Classical Physics

**Methodology**: Sketch → identify conserved quantities → governing equations →
estimate order-of-magnitude → solve → check dimensions + limiting cases.

**ODE-based simulation**:
```python
from scipy.integrate import solve_ivp

# Harmonic oscillator: d²x/dt² = -k/m * x
def harmonic(t, y, k=1.0, m=1.0):
    x, v = y
    return [v, -k/m * x]

sol = solve_ivp(harmonic, [0, 20], [1.0, 0.0], method='RK45', dense_output=True)
```

**Dimensional analysis**: Buckingham π theorem — n variables, k dimensions → n-k
dimensionless groups. Always check: units, limiting cases (v<<c, λ<<L, ℏ→0).

### Quantum Mechanics

**Postulates**: State |ψ⟩ ∈ H; Observables Â (Hermitian); Born rule P(a)=|⟨a|ψ⟩|²;
Schrödinger iℏ∂|ψ⟩/∂t=Ĥ|ψ⟩; Composite H=H_A⊗H_B.

**Key**: Entanglement |Φ⁺⟩=(|00⟩+|11⟩)/√2; Bell |S|≤2 (classical), 2√2 (quantum);
Decoherence via Lindblad equation; Density matrix ρ=Σp_i|ψ_i⟩⟨ψ_i|.

---

## Part VI: Bayesian Inference

**Workflow**: Priors → model specification → MCMC sampling (4+ chains, NUTS) →
diagnostics (R-hat<1.01, ESS>400, 0 divergences) → PPC → sensitivity → report.

| Metric | Threshold | Catches |
|--------|-----------|---------|
| R-hat | < 1.01 | Chains not mixed |
| ESS | > 400 | Too few effective samples |
| Divergences | 0 | Bad HMC geometry |

**Comparison**: WAIC, LOO-CV (PSIS), Bayes factor (sensitive to priors).
**Tools**: PyMC, Stan, NumPyro, ArviZ.

---

## Part VII: Cross-Domain Integration

### Quantum Chemistry Pipeline
```
Molecular geometry (RDKit) → Hamiltonian (PennyLane qchem) →
qubit mapping (Jordan-Wigner / Bravyi-Kitaev) → symmetry reduction (tapering) →
VQE (PennyLane / Qiskit) → optimizer (COBYLA / L-BFGS-B from SciPy) →
error mitigation (ZNE) → ground state energy
```

### Drug Discovery Pipeline
```
Target protein (BioPython PDB) → active site → virtual screening (RDKit) →
docking → MD simulation → free energy → ADMET (RDKit descriptors) → lead optimization
```

### Genomics + Statistics
```
Raw reads → QC + align → variant call → feature engineering →
statistical testing (SciPy stats) → Bayesian model (PyMC) → ML classification
```

---

## Failure Modes

| Failure | Recovery |
|---------|----------|
| Barren plateau (quantum) | Better ansatz; local cost; layer-wise training |
| Noise dominates | Error mitigation; shorter circuit |
| Classical beats quantum | Report honestly; classical may be correct |
| SCF doesn't converge | Better initial guess; different functional |
| MD crashes | Smaller timestep; constraint bonds |
| R-hat > 1.01 | Run longer; reparameterise; simpler model |
| Divergences | target_accept → 0.99; non-centered parameterisation |
| Low alignment rate | Check adapters, rRNA, quality |
| Batch effects | ComBat; balanced design |
| Optimization stuck | Try different method; bounds; better x0 |

---

## Tools Summary

| Domain | Tools |
|--------|-------|
| **Quantum** | Qiskit, PennyLane, Cirq, cuQuantum |
| **Biology** | BioPython, samtools, GATK, STAR, DESeq2, Seurat, Scanpy, AlphaFold2 |
| **Chemistry** | RDKit, Gaussian/ORCA, GROMACS/AMBER, AutoDock, PyMOL |
| **Scientific** | SciPy, NumPy, SymPy, Matplotlib, pandas |
| **Bayesian** | PyMC, Stan, NumPyro, ArviZ, emcee |
| **Pipelines** | Snakemake, Nextflow |

---

## Anti-Patterns Summary

| Domain | Anti-Pattern | Fix |
|--------|-------------|-----|
| Quantum | No classical baseline | Always run classical first |
| Quantum | Deep circuits on NISQ | Stay shallow (<100 gates) |
| Quantum | No error mitigation | Always mitigate noise |
| Biology | No QC before analysis | QC raw + processed |
| Biology | No multiple testing correction | FDR (BH) for omics |
| Chemistry | Wrong method for system size | Use hierarchy (ab initio→DFT→FF) |
| Chemistry | MD without equilibration | Equilibrate before production |
| SciPy | No method in minimize() | Choose method explicitly |
| SciPy | p-value without effect size | Report effect size + CI |
| Bayesian | Ignoring divergences | Fix geometry before trusting |
| Bayesian | Flat priors without justification | Use weakly informative priors |

---

## Citations

- Nielsen & Chuang, "Quantum Computation and Quantum Information"
- Preskill, "Quantum Computing in the NISQ era and beyond", 2018
- Cerezo et al., "Variational Quantum Algorithms", Nature Reviews Physics 2021
- Cramer, "Essentials of Computational Chemistry"
- Love et al., "Moderated estimation of fold change", Genome Biology 2014
- Jumper et al., "Highly accurate protein structure prediction", Nature 2021
- Virtanen et al., "SciPy 1.0", Nature Methods 2020
- Landrum et al., "RDKit: Open-source cheminformatics"


## Implementation Classes

### QuantumCircuitBuilder: Qiskit Circuit Construction

```python
from qiskit import QuantumCircuit, QuantumRegister, ClassicalRegister, transpile
from qiskit_aer import AerSimulator
from qiskit.quantum_info import Statevector
from typing import List, Optional, Dict
import numpy as np


class QuantumCircuitBuilder:
    """Wraps Qiskit's QuantumCircuit for programmatic circuit construction.

    Provides a fluent API for building quantum circuits with standard gates,
    measurement, and simulation. Wraps Qiskit's transpiler and AerSimulator
    for execution on statevector backend.
    """

    def __init__(self, n_qubits: int, n_classical: int = None,
                 name: str = "circuit"):
        self.n_qubits = n_qubits
        self.n_classical = n_classical or n_qubits
        self._qr = QuantumRegister(n_qubits, "q")
        self._cr = ClassicalRegister(self.n_classical, "c")
        self._circuit = QuantumCircuit(self._qr, self._cr, name=name)
        self._counts = None

    def add_hadamard(self, qubit: int) -> "QuantumCircuitBuilder":
        """Apply Hadamard gate: creates equal superposition |+> = (|0>+|1>)/sqrt(2)."""
        self._circuit.h(self._qr[qubit])
        return self

    def add_cnot(self, control: int, target: int) -> "QuantumCircuitBuilder":
        """Apply CNOT (controlled-X) gate: entangles control and target qubits."""
        self._circuit.cx(self._qr[control], self._qr[target])
        return self

    def add_rotation(self, qubit: int, axis: str,
                     theta: float) -> "QuantumCircuitBuilder":
        """Apply rotation gate around specified axis.

        Args:
            qubit: Target qubit index
            axis: 'x', 'y', or 'z'
            theta: Rotation angle in radians
        """
        gate_map = {
            "x": self._circuit.rx,
            "y": self._circuit.ry,
            "z": self._circuit.rz,
        }
        if axis.lower() not in gate_map:
            raise ValueError(f"Axis must be 'x', 'y', or 'z', got '{axis}'")
        gate_map[axis.lower()](theta, self._qr[qubit])
        return self

    def measure(self, qubit: int = None,
                classical_bit: int = None) -> "QuantumCircuitBuilder":
        """Measure qubit(s) into classical bit(s).

        If no arguments, measure all qubits into corresponding classical bits.
        """
        if qubit is not None:
            self._circuit.measure(self._qr[qubit],
                                 self._cr[classical_bit or qubit])
        else:
            self._circuit.measure(self._qr, self._cr)
        return self

    def run_simulator(self, shots: int = 1024,
                      seed: int = None) -> Dict[str, int]:
        """Execute circuit on AerSimulator statevector backend.

        Args:
            shots: Number of measurement shots
            seed: Random seed for reproducibility
        """
        simulator = AerSimulator()
        compiled = transpile(self._circuit, simulator)
        job = simulator.run(compiled, shots=shots, seed_simulator=seed)
        self._counts = job.result().get_counts()
        return self._counts

    def get_counts(self) -> Dict[str, int]:
        """Return measurement counts from last simulation run."""
        if self._counts is None:
            raise RuntimeError("Must call run_simulator() first.")
        return self._counts

    def get_statevector(self) -> np.ndarray:
        """Get the statevector before measurement (strips measurement gates)."""
        sv = Statevector.from_instruction(
            self._circuit.remove_final_measurements(inplace=False)
        )
        return sv.data

    @property
    def circuit(self) -> QuantumCircuit:
        """Access the underlying Qiskit QuantumCircuit."""
        return self._circuit

    def draw(self, output: str = "text") -> str:
        """Draw the circuit in specified format ('text', 'mpl', 'latex')."""
        return str(self._circuit.draw(output=output))
```

### VQESolver: PennyLane Variational Eigensolver

```python
import numpy as np
from typing import List, Optional, Dict, Tuple
import pennylane as qml


class VQESolver:
    """Wraps PennyLane for Variational Quantum Eigensolver.

    VQE finds the ground state energy of a molecular Hamiltonian by
    optimizing a parameterized quantum circuit (ansatz) to minimize
    <psi(theta)|H|psi(theta)>.
    """

    def __init__(self, n_qubits: int, n_layers: int = 2,
                 device: str = "default.qubit"):
        self.n_qubits = n_qubits
        self.n_layers = n_layers
        self._dev = qml.device(device, wires=n_qubits)
        self._hamiltonian = None
        self._params = None
        self._energy = None

    def build_hamiltonian(self, coeffs: List[float],
                          paulis: List[str]) -> None:
        """Build Hamiltonian from Pauli string representation.

        Args:
            coeffs: Coefficients for each Pauli term
            paulis: Pauli strings (e.g., "ZZII", "XIII")

        Example:
            H = 0.5 * Z x Z + 0.3 * X x I + ...
        """
        obs_map = {
            "I": qml.Identity, "X": qml.PauliX,
            "Y": qml.PauliY, "Z": qml.PauliZ,
        }

        terms = []
        for coeff, pauli_str in zip(coeffs, paulis):
            ops = []
            for wire, char in enumerate(pauli_str):
                if char != "I":
                    ops.append(obs_map[char](wire))
            if ops:
                term = ops[0]
                for op in ops[1:]:
                    term = term @ op
                terms.append(term)

        self._hamiltonian = qml.Hamiltonian(coeffs, terms)

    def build_ansatz(self, params: np.ndarray = None) -> None:
        """Initialize variational ansatz parameters.

        Uses a hardware-efficient ansatz with RY rotations and
        entangling CNOT layers. Parameters shape: (n_layers, n_qubits).
        """
        if params is None:
            params = np.random.randn(self.n_layers, self.n_qubits) * 0.1
        self._params = params

    def _circuit(self, params):
        """The variational quantum circuit definition."""
        for layer in range(self.n_layers):
            for qubit in range(self.n_qubits):
                qml.RY(params[layer, qubit], wires=qubit)
            for qubit in range(self.n_qubits - 1):
                qml.CNOT(wires=[qubit, qubit + 1])
        return qml.expval(self._hamiltonian)

    def optimize(self, steps: int = 200, lr: float = 0.1,
                 optimizer: str = "adam") -> Tuple[float, np.ndarray]:
        """Run VQE optimization to find ground state energy.

        Args:
            steps: Number of optimization steps
            lr: Learning rate
            optimizer: 'adam', 'gd', 'adagrad', or 'rmsprop'
        """
        if self._hamiltonian is None:
            raise RuntimeError("Must call build_hamiltonian() first.")
        if self._params is None:
            self.build_ansatz()

        opt_map = {
            "adam": qml.AdamOptimizer,
            "gd": qml.GradientDescentOptimizer,
            "adagrad": qml.AdagradOptimizer,
            "rmsprop": qml.RMSPropOptimizer,
        }

        qnode = qml.QNode(self._circuit, self._dev)
        opt = opt_map.get(optimizer, qml.AdamOptimizer)(stepsize=lr)

        params = self._params.copy()
        energies = []

        for step in range(steps):
            params, energy = opt.step_and_cost(qnode, params)
            energies.append(energy)

        self._params = params
        self._energy = energies[-1]
        return self._energy, params

    def get_ground_state(self) -> Dict[str, float]:
        """Return VQE results: ground state energy and optimal parameters."""
        if self._energy is None:
            raise RuntimeError("Must call optimize() first.")
        return {
            "ground_state_energy": float(self._energy),
            "optimal_params": self._params.tolist(),
            "n_qubits": self.n_qubits,
            "n_layers": self.n_layers,
        }
```

### MolecularAnalyzer: RDKit Cheminformatics

```python
import numpy as np
from typing import List, Optional, Dict, Any, Tuple
from rdkit import Chem
from rdkit.Chem import Descriptors, AllChem, DataStructs
from rdkit.Chem.Fingerprints import FingerprintMols
from rdkit.Chem import rdMolDescriptors


class MolecularAnalyzer:
    """Wraps RDKit for molecular analysis: descriptors, fingerprints,
    substructure search, and similarity computation.
    """

    def __init__(self):
        self._mol = None
        self._smiles = None

    def load_molecule(self, smiles: str) -> "MolecularAnalyzer":
        """Load molecule from SMILES string with sanitization."""
        self._mol = Chem.MolFromSmiles(smiles)
        if self._mol is None:
            raise ValueError(f"Invalid SMILES: '{smiles}'")
        self._smiles = smiles
        return self

    def get_descriptors(self) -> Dict[str, float]:
        """Compute key molecular descriptors (MW, LogP, TPSA, HBA, HBD, etc.)."""
        if self._mol is None:
            raise RuntimeError("Must call load_molecule() first.")

        return {
            "molecular_weight": Descriptors.MolWt(self._mol),
            "logp": Descriptors.MolLogP(self._mol),
            "tpsa": Descriptors.TPSA(self._mol),
            "hba": Descriptors.NumHAcceptors(self._mol),
            "hbd": Descriptors.NumHDonors(self._mol),
            "rotatable_bonds": Descriptors.NumRotatableBonds(self._mol),
            "aromatic_rings": Descriptors.NumAromaticRings(self._mol),
            "heavy_atom_count": Descriptors.HeavyAtomCount(self._mol),
            "fraction_csp3": Descriptors.FractionCSP3(self._mol),
            "ring_count": Descriptors.RingCount(self._mol),
        }

    def get_fingerprints(self, fp_type: str = "morgan",
                         radius: int = 2,
                         n_bits: int = 2048) -> np.ndarray:
        """Compute molecular fingerprint as numpy bit vector.

        Args:
            fp_type: 'morgan' (ECFP), 'maccs', 'rdkit', 'atom_pair'
            radius: Morgan fingerprint radius (default 2 = ECFP4)
            n_bits: Bit vector length
        """
        if self._mol is None:
            raise RuntimeError("Must call load_molecule() first.")

        if fp_type == "morgan":
            fp = AllChem.GetMorganFingerprintAsBitVect(
                self._mol, radius, nBits=n_bits,
            )
        elif fp_type == "maccs":
            fp = rdMolDescriptors.GetMACCSKeysFingerprint(self._mol)
        elif fp_type == "rdkit":
            fp = FingerprintMols.FingerprintMol(self._mol, fpSize=n_bits)
        elif fp_type == "atom_pair":
            fp = rdMolDescriptors.GetAtomPairFingerprint(self._mol)
            arr = np.array(fp.GetNonzeroElements(), dtype=np.int8)
            return arr
        else:
            raise ValueError(f"Unknown fp_type: '{fp_type}'")

        arr = np.zeros((n_bits if fp_type != "maccs" else 167,), dtype=np.int8)
        DataStructs.ConvertToNumpyArray(fp, arr)
        return arr

    def find_substructures(self, smarts: str) -> List[Tuple[int, ...]]:
        """Find substructure matches using SMARTS pattern.

        Returns list of atom index tuples for each match.
        """
        if self._mol is None:
            raise RuntimeError("Must call load_molecule() first.")

        pattern = Chem.MolFromSmarts(smarts)
        if pattern is None:
            raise ValueError(f"Invalid SMARTS pattern: '{smarts}'")

        matches = self._mol.GetSubstructMatches(pattern)
        return list(matches)

    def compute_similarity(self, other_smiles: str,
                           fp_type: str = "morgan",
                           metric: str = "tanimoto") -> float:
        """Compute molecular similarity using fingerprints.

        Args:
            other_smiles: SMILES of the comparison molecule
            fp_type: Fingerprint type for comparison
            metric: 'tanimoto', 'dice', or 'cosine'
        """
        if self._mol is None:
            raise RuntimeError("Must call load_molecule() first.")

        other_mol = Chem.MolFromSmiles(other_smiles)
        if other_mol is None:
            raise ValueError(f"Invalid comparison SMILES: '{other_smiles}'")

        fp1 = AllChem.GetMorganFingerprintAsBitVect(self._mol, 2, nBits=2048)
        fp2 = AllChem.GetMorganFingerprintAsBitVect(other_mol, 2, nBits=2048)

        metric_map = {
            "tanimoto": DataStructs.TanimotoSimilarity,
            "dice": DataStructs.DiceSimilarity,
            "cosine": DataStructs.CosineSimilarity,
        }

        if metric not in metric_map:
            raise ValueError(f"Unknown metric: '{metric}'")

        return float(metric_map[metric](fp1, fp2))
```

### SequenceAnalyzer: BioPython Sequence Analysis

```python
import numpy as np
from typing import List, Optional, Dict, Any, Tuple
from Bio import SeqIO, Align
from Bio.SeqRecord import SeqRecord
from Bio.Phylo.TreeConstruction import DistanceCalculator, DistanceTreeConstructor
from Bio import AlignIO
from io import StringIO


class SequenceAnalyzer:
    """Wraps BioPython for sequence analysis: FASTA loading, alignment,
    phylogenetic tree construction, and GC content computation.
    """

    def __init__(self):
        self._records = []
        self._alignment = None

    def load_fasta(self, filepath: str = None,
                   fasta_string: str = None) -> List[SeqRecord]:
        """Load sequences from FASTA file or string.

        Args:
            filepath: Path to FASTA file
            fasta_string: Raw FASTA format string
        """
        if filepath:
            self._records = list(SeqIO.parse(filepath, "fasta"))
        elif fasta_string:
            self._records = list(SeqIO.parse(StringIO(fasta_string), "fasta"))
        else:
            raise ValueError("Must provide filepath or fasta_string")

        return self._records

    def align_sequences(self, method: str = "global") -> Any:
        """Perform pairwise or multiple sequence alignment.

        Args:
            method: 'global' (Needleman-Wunsch) or 'local' (Smith-Waterman)
        """
        if not self._records:
            raise RuntimeError("Must load sequences first.")

        if len(self._records) == 2:
            aligner = Align.PairwiseAligner()
            aligner.mode = method
            aligner.substitution_matrix = Align.substitution_matrices.load(
                "BLOSUM62",
            )
            alignments = aligner.align(self._records[0].seq,
                                       self._records[1].seq)
            return alignments[0]
        else:
            # Multiple sequence alignment via progressive approach
            from Bio.Align import MultipleSeqAlignment
            self._alignment = MultipleSeqAlignment(self._records)
            return self._alignment

    def build_phylogenetic_tree(self, model: str = "identity") -> Any:
        """Build phylogenetic tree from loaded sequences.

        Uses neighbor-joining on pairwise distances computed from
        the multiple sequence alignment.

        Args:
            model: Distance model ('identity', 'blastn', 'trans')
        """
        if not self._records:
            raise RuntimeError("Must load sequences first.")
        if len(self._records) < 3:
            raise ValueError("Need at least 3 sequences for a meaningful tree.")

        if self._alignment is None:
            self.align_sequences()

        calculator = DistanceCalculator(model)
        dm = calculator.get_distance(self._alignment)

        constructor = DistanceTreeConstructor()
        tree = constructor.nj(dm)

        return tree

    def compute_gc_content(self) -> Dict[str, float]:
        """Compute GC content for each loaded sequence.

        GC content = (G + C) / total_length. Higher GC implies more
        thermally stable DNA.
        """
        if not self._records:
            raise RuntimeError("Must load sequences first.")

        results = {}
        for record in self._records:
            seq = str(record.seq).upper()
            gc_count = seq.count("G") + seq.count("C")
            total = len(seq)
            results[record.id] = gc_count / total if total > 0 else 0.0

        return results
```

### BayesianModel: PyMC Bayesian Inference

```python
import numpy as np
import pymc as pm
import arviz as az
from typing import Optional, Dict, Any


class BayesianModel:
    """Wraps PyMC for Bayesian statistical modeling with full MCMC workflow.

    Supports prior specification, posterior sampling, convergence diagnostics,
    and posterior predictive checks. Uses PyMC's context manager pattern.
    """

    def __init__(self, name: str = "bayesian_model"):
        self.name = name
        self._model = None
        self._idata = None
        self._priors = {}

    def specify_priors(self, priors: Dict[str, Dict[str, Any]]) -> "BayesianModel":
        """Specify prior distributions for model parameters.

        Args:
            priors: Dict mapping parameter name to distribution spec.
                    Example: {"beta": {"dist": "Normal", "mu": 0, "sigma": 5},
                              "sigma": {"dist": "HalfNormal", "sigma": 2}}

        Supported: Normal, HalfNormal, StudentT, Uniform, Gamma,
        Exponential, Beta, LogNormal.
        """
        dist_map = {
            "Normal": pm.Normal,
            "HalfNormal": pm.HalfNormal,
            "StudentT": pm.StudentT,
            "Uniform": pm.Uniform,
            "Gamma": pm.Gamma,
            "Exponential": pm.Exponential,
            "Beta": pm.Beta,
            "LogNormal": pm.LogNormal,
        }

        self._priors = {}
        for pname, spec in priors.items():
            dist_class = dist_map.get(spec["dist"])
            if dist_class is None:
                raise ValueError(f"Unknown distribution: '{spec['dist']}'")
            params = {k: v for k, v in spec.items() if k != "dist"}
            self._priors[pname] = {"dist": dist_class, "params": params}

        return self

    def sample_posterior(self, y: np.ndarray, X: np.ndarray = None,
                         draws: int = 2000, tune: int = 1000,
                         chains: int = 4,
                         target_accept: float = 0.9) -> Any:
        """Build model and sample posterior with NUTS.

        If X is provided, fits a linear model: y ~ Normal(alpha + X @ beta, sigma).
        Otherwise samples from the specified priors on y directly.
        """
        with pm.Model(name=self.name) as model:
            self._model = model

            params = {}
            for pname, pspec in self._priors.items():
                params[pname] = pspec["dist"](pname, **pspec["params"])

            if X is not None:
                alpha = params.get("alpha", pm.Flat("alpha"))
                beta = params.get("beta", pm.Flat("beta", shape=X.shape[1]))
                mu = alpha + pm.math.dot(X, beta)
            else:
                mu = params.get("mu", pm.Flat("mu"))

            sigma = params.get("sigma", pm.HalfNormal("sigma", sigma=1))
            pm.Normal("y_obs", mu=mu, sigma=sigma, observed=y)

            self._idata = pm.sample(
                draws=draws, tune=tune, chains=chains,
                target_accept=target_accept,
            )

        return self._idata

    def diagnose_convergence(self) -> Dict[str, Any]:
        """Run MCMC convergence diagnostics.

        Checks R-hat (Gelman-Rubin) < 1.01, ESS > 400,
        and zero divergent transitions.
        """
        if self._idata is None:
            raise RuntimeError("Must call sample_posterior() first.")

        summary = az.summary(self._idata)

        rhat_values = summary["r_hat"]
        ess_bulk = summary["ess_bulk"]
        ess_tail = summary["ess_tail"]

        n_divergences = 0
        if "diverging" in self._idata.sample_stats:
            n_divergences = int(
                self._idata.sample_stats["diverging"].sum().values
            )

        return {
            "rhat_ok": bool((rhat_values < 1.01).all()),
            "rhat_max": float(rhat_values.max()),
            "ess_bulk_min": float(ess_bulk.min()),
            "ess_tail_min": float(ess_tail.min()),
            "ess_ok": bool((ess_bulk > 400).all() and (ess_tail > 400).all()),
            "n_divergences": n_divergences,
            "converged": bool(
                (rhat_values < 1.01).all()
                and (ess_bulk > 400).all()
                and n_divergences == 0
            ),
            "summary": summary,
        }

    def posterior_predictive_check(self, X: np.ndarray = None,
                                   n_samples: int = 500) -> Dict[str, Any]:
        """Generate posterior predictive samples for model checking.

        PPC compares observed data against data simulated from the posterior.
        Good fit means simulated data resembles observed data.
        """
        if self._idata is None or self._model is None:
            raise RuntimeError("Must call sample_posterior() first.")

        with self._model:
            if X is not None:
                pm.set_data({"X": X})
            ppc = pm.sample_posterior_predictive(
                self._idata, predictions=True,
            )

        preds = ppc.predictions["y_obs"].values
        preds_flat = preds.reshape(-1, preds.shape[-1])

        return {
            "mean": preds_flat.mean(axis=0),
            "std": preds_flat.std(axis=0),
            "ci_lower": np.percentile(preds_flat, 2.5, axis=0),
            "ci_upper": np.percentile(preds_flat, 97.5, axis=0),
            "samples": preds_flat[:n_samples],
        }
```

---

## Verification Checklist

- [ ] Experimental design reviewed for statistical validity
- [ ] Multiple testing correction applied if needed
- [ ] Reproducibility ensured (random seeds, version pinning)
- [ ] Simulation results validated against analytical solutions
- [ ] Quantum circuits optimized (gate count, depth)
- [ ] Chemical/biological results validated against literature
- [ ] Error bars / confidence intervals reported
- [ ] Computational cost documented
- [ ] Data provenance tracked
- [ ] Results peer-reviewed or independently verified


---

## Decision tree

See **When to use** above.

## Workflow

1. Read the skill body above.
2. Identify the relevant section for your task.
3. Apply the patterns and examples provided.
4. Verify against the listed anti-patterns.

## Examples

Examples are interleaved throughout the skill body above.

## Anti-patterns

See the **When to use** criteria above. The skill is *not* applied when:
- The task is outside the skill's declared scope.
- Simpler alternatives exist.

## Failure modes

- **Misapplied scope**: invoking the skill for tasks it doesn't cover.
- **Outdated reference**: real-world library APIs may have shifted since synthesis.
- **Pattern drift**: the skill's patterns describe idealized APIs, not exact production code.

## Related skills

- `machine-learning-pipelines`
- `reinforcement-learning-pipelines`

