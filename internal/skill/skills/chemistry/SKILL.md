# Skill: chemistry

> Molecular modelling (DFT, MD, force fields), reactions,
> kinetics, spectroscopy. Method choice must match system
> size and accuracy need.

## Decision tree

```
Chemistry problem
        │
        ▼
[Step 1] Define system + property of interest
        │
        ▼
[Step 2] Choose method (DFT / ab initio / MD / force field)
        │
        ▼
[Step 3] Choose basis set / software
        │
        ▼
[Step 4] Compute + check convergence
        │
        ▼
[Step 5] Validate vs experiment
        │
        ▼
[Step 6] Report with uncertainty
```

## Workflow

### Method selection

| Method | System size | Accuracy | Cost |
|--------|-------------|----------|------|
| **Ab initio (CCSD(T))** | <10 atoms | Excellent (gold standard) | Very high |
| **DFT** | 10-1000 atoms | Good | Medium |
| **Semi-empirical (PM7)** | 100-1000 atoms | OK | Low |
| **Force field (MM)** | >1000 atoms | Geometry / dynamics | Low |
| **Coarse-grained** | >10⁶ atoms | Trends | Very low |

**Ab initio methods**:
- HF (Hartree-Fock): baseline; no correlation
- MP2: perturbative correlation
- CCSD(T): gold standard; expensive
- CI, FCI: exact (small systems only)

**DFT functionals**:
- LDA: simplest; underestimates gaps
- GGA (PBE, BLYP): workhorse
- Hybrid (B3LYP, PBE0): better for many properties
- Range-separated (ωB97X-D): better for long-range
- Dispersion-corrected (DFT-D3): van der Waals

### Basis sets

| Basis | Use |
|-------|-----|
| **STO-3G** | Minimal; teaching |
| **3-21G, 6-31G** | Split-valence; small molecules |
| **6-31G*, 6-311G** | Polarisation; geometry |
| **cc-pVTZ, cc-pVQZ** | Correlation-consistent; high accuracy |
| **aug-cc-pVTZ** | Diffuse functions; anions, excited states |
| **LANL2DZ** | Effective core potential; heavy elements |

### Molecular dynamics (MD)

```
Newton's equations: F = ma
F = -∇U (force from potential)
U = force field (e.g. AMBER, CHARMM, OPLS)
```

**Workflow**:
1. Minimise energy (geometry optimisation)
2. Equilibrate (NVT, then NPT)
3. Production run (10-100 ns typical)
4. Analyse (RMSD, RMSF, hydrogen bonds, free energy)

**Integrators**: Verlet, leapfrog; 1-2 fs time step.

### Reaction kinetics

**Rate law**: v = k [A]^m [B]^n

**Arrhenius**: k = A exp(-Ea/RT)

**Transition state theory**: k = (kT/h) exp(-ΔG‡/RT)

**Mechanism elucidation**: identify TS; IRC; kinetic isotope effects.

### Thermochemistry

**Enthalpy**: H = U + PV
**Entropy**: S = k ln W
**Gibbs free energy**: G = H - TS

**For a reaction**: ΔG = ΔH - TΔS

Equilibrium constant: K = exp(-ΔG/RT)

### Spectroscopy

| Type | Probes | Application |
|------|--------|-------------|
| **NMR** | Local chemical environment | Structure |
| **IR** | Vibrations | Functional groups |
| **UV-Vis** | Electronic transitions | Conjugation |
| **Mass spec** | Molecular weight | Identification |
| **X-ray** | Crystal structure | 3D structure |
| **EPR** | Unpaired electrons | Radicals |

### Drug discovery workflow

1. **Target identification** (protein, pathway)
2. **Hit identification** (HTS, virtual screening)
3. **Lead optimisation** (SAR, ADMET)
4. **Preclinical** (in vitro / in vivo)
5. **IND** (regulatory submission)

**Computational tools**:
- Docking (AutoDock, Glide)
- MD (GROMACS, AMBER, NAMD)
- Free energy (FEP, TI)
- ADMET prediction (RDKit, admetSAR)

## Examples

### Example 1: reaction energetics (DFT)

```
Reaction: A + B → C
DFT: B3LYP/6-31G*
H(A) = -230.5 Ha; H(B) = -76.4 Ha; H(C) = -307.1 Ha
ΔH = -307.1 - (-230.5 - 76.4) = -0.2 Ha = -126 kcal/mol
ΔG = ΔH - TΔS (compute from vibrational freq)
Result: exothermic, spontaneous
```

### Example 2: protein-ligand binding (MD)

```
Protein: 200 residues; ligand: small molecule
Force field: AMBER ff14SB + GAFF
Solvation: TIP3P water + 0.15 M NaCl
Equilibration: 5 ns NPT
Production: 100 ns
Binding free energy: -8.5 ± 0.6 kcal/mol (MM-PBSA)
Compare to ITC experiment: -9.1 ± 0.2 kcal/mol
```

### Example 3: catalyst screening (DFT)

```
Reaction: hydrogenation
Catalysts: 12 transition metal complexes
Property: activation energy Ea
DFT: PBE0/def2-TZVP
Top: Complex 7 (Ea = 12 kcal/mol)
Bottom: Complex 3 (Ea = 28 kcal/mol)
Predict: Complex 7 fastest at RT
```

## Anti-patterns

### ❌ Wrong method for system size

Ab initio for protein = intractable. Use hierarchy.

### ❌ No basis set convergence

Results depend on basis. Check: compute with larger basis; compare.

### ❌ MD without equilibration

Averaging pre-equilibrium data. Always equilibrate first.

### ❌ Docking without validation

Known ligands as controls (RMSD vs crystal pose).

### ❌ Single-point calculations on bad geometry

Optimise first; single-point on bad geometry is meaningless.

### ❌ Ignoring solvent

Gas-phase calculations ≠ solution. Implicit (PCM) or explicit.

## Failure modes

| Failure | Recovery |
|---------|----------|
| SCF doesn't converge | Better initial guess; different functional; damping |
| Basis set too small | Larger basis; check convergence |
| MD simulation crashes | Smaller time step; constraint bonds; better parameters |
| Free energy wrong | Longer sampling; enhanced sampling (metadynamics, replica exchange) |
| Docking wrong pose | Constrain known binding site; use multiple algorithms |
| Solvent effects matter | Explicit solvent; larger box; longer equilibration |

## Related skills

| Skill | When to chain |
|-------|---------------|
| `/quantum-physics` | Underlying QM formalism |
| `/biology` | Biomolecules; drug discovery |
| `/quantum-ml` | VQE for chemistry |
| `/physics` | Classical limit; transport |

## Tools

| Tool | Purpose |
|------|---------|
| **Gaussian / ORCA** | Quantum chemistry |
| **Q-Chem** | DFT, ab initio |
| **GROMACS / AMBER / NAMD** | MD |
| **AutoDock / Glide** | Docking |
| **RDKit** | Cheminformatics |
| **PyMOL / ChimeraX** | Visualisation |
| **Schrödinger suite** | Integrated drug discovery |

## Citations

- Cramer, "Essentials of Computational Chemistry"
- Jensen, "Introduction to Computational Chemistry"
- Leach, "Molecular Modelling: Principles and Applications"