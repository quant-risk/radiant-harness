# Skill: physics

> Classical physics: mechanics, EM, thermodynamics, statistical
> mechanics, optics, waves, fluids. Dimensional analysis +
> limiting cases + order-of-magnitude estimates before
> computation.

## Decision tree

```
Physics problem
        │
        ▼
[Step 1] Identify subfield (mechanics / EM / thermo / fluids / ...)
        │
        ▼
[Step 2] Set up coordinate system + variables
        │
        ▼
[Step 3] Identify relevant conservation laws + governing equations
        │
        ▼
[Step 4] Order-of-magnitude estimate
        │
        ▼
[Step 5] Solve (analytical / numerical)
        │
        ▼
[Step 6] Dimensional check + limiting cases
        │
        ▼
[Step 7] Sanity check vs experiment or known result
```

## Workflow

### Problem-solving methodology

1. **Sketch the situation** — coordinate system, forces, fields
2. **Identify what's conserved** — energy, momentum, mass, charge
3. **Pick governing equations** — Newton's, Maxwell's, Navier-Stokes
4. **Estimate first** — order-of-magnitude before computation
5. **Solve** — analytical (exact) or numerical (approximate)
6. **Check** — dimensions, limiting cases, units
7. **Sanity check** — vs intuition, prior result, experiment

### Mechanics

**Newton's laws**: F = ma; F₁₂ = -F₂₁ (action-reaction)

**Conservation**:
- Energy: KE + PE = const (no friction)
- Momentum: p = mv (vector); Σ p = const (no external force)
- Angular momentum: L = r × p; Σ L = const (no external torque)

**Common problems**:
- Projectile motion: parabolic trajectory
- Oscillations: x(t) = A cos(ωt + φ); ω = √(k/m)
- Central force: orbits (Kepler's laws)
- Rigid body: rotation + translation

**Limiting cases**:
- v << c → Newtonian (vs relativistic)
- λ << L → geometric optics (vs wave)
- ℏ → 0 → classical (vs quantum)

### Electromagnetism

**Maxwell's equations** (in vacuum):
- ∇·E = ρ/ε₀ (Gauss)
- ∇·B = 0 (no monopoles)
- ∇×E = -∂B/∂t (Faraday)
- ∇×B = μ₀J + μ₀ε₀∂E/∂t (Ampère-Maxwell)

**Common problems**:
- Coulomb's law: F = kq₁q₂/r²
- Capacitor: C = ε₀A/d; U = ½CV²
- Inductor: V = L dI/dt; U = ½LI²
- RC / RL / RLC circuits

**Lorentz force**: F = q(E + v × B)

### Thermodynamics

**Laws**:
- 0th: thermal equilibrium is transitive
- 1st: dU = δQ - δW (energy conservation)
- 2nd: dS ≥ δQ/T (entropy non-decreasing)
- 3rd: S → 0 as T → 0 (perfect crystal)

**Processes**:
- Isothermal: T = const
- Adiabatic: Q = 0; PV^γ = const
- Isobaric: P = const
- Isochoric: V = const

**Cycles**:
- Carnot efficiency: η = 1 - T_cold/T_hot
- Otto / Diesel / Brayton cycles

### Statistical mechanics

**Boltzmann distribution**: P(E) ∝ exp(-E/kT)

**Partition function**: Z = Σ exp(-E_i/kT)
- F = -kT ln Z (Helmholtz free energy)
- U = -∂ ln Z / ∂β (internal energy)
- S = k(ln Z + βU) (entropy)

**Distributions**:
- Maxwell-Boltzmann: classical, distinguishable
- Bose-Einstein: bosons (integer spin)
- Fermi-Dirac: fermions (half-integer spin)

### Optics

**Geometric**: Snell's law n₁ sin θ₁ = n₂ sin θ₂
**Wave**: λ f = c; interference, diffraction
**Quantum**: photon energy E = hν

### Fluid dynamics

**Navier-Stokes**: ρ(∂v/∂t + v·∇v) = -∇P + μ∇²v + ρg

**Reynolds number**: Re = ρvL/μ — inertial vs viscous
- Re << 1: laminar (Stokes flow)
- Re >> 1: turbulent

**Bernoulli**: P + ½ρv² + ρgh = const (inviscid, steady)

### Dimensional analysis

**Buckingham π theorem**: 
- n variables, k dimensions → n - k dimensionless groups
- Express answer in terms of π groups

Example: pendulum period T = f(L, g, m)
- [T] = s; [L] = m; [g] = m/s²
- One π group: T √(g/L) = const
- T = 2π √(L/g) (verified experimentally)

## Examples

### Example 1: projectile motion

```
Initial: v₀ = 50 m/s, θ = 30°, g = 9.81 m/s²
Range: R = v₀² sin(2θ)/g = 2500 × sin(60°)/9.81 = 220 m
Max height: H = v₀² sin²(θ)/(2g) = 2500 × 0.25/19.62 = 31.9 m
Time of flight: T = 2v₀ sin(θ)/g = 100 × 0.5/9.81 = 5.1 s
Limiting cases:
  - θ → 0: R → 0 (correct)
  - θ = 45°: R = v₀²/g = 255 m (max range; correct)
```

### Example 2: RC circuit

```
V_in(t) = V₀ for t > 0 (step)
V_C(t) = V₀ (1 - e^(-t/RC))
Time constant: τ = RC
Charging 99%: 5τ
Limiting:
  - t → 0: V_C → 0 (correct)
  - t → ∞: V_C → V₀ (correct)
  - R → 0: V_C rises instantly (correct)
```

### Example 3: Carnot efficiency

```
T_hot = 600 K, T_cold = 300 K
η = 1 - 300/600 = 50%
Limiting:
  - T_cold → 0: η → 1 (correct, but unphysical 3rd law)
  - T_hot = T_cold: η = 0 (correct)
```

## Anti-patterns

### ❌ Wrong units

SI vs CGS vs imperial. Always declare unit system; convert inputs.

### ❌ No limiting case check

Model fails at edge. Always check: low / high / extreme parameter.

### ❌ Failing to estimate

Compute without order-of-magnitude check. Compute blindly; find errors
later.

### ❌ Confusing scalars / vectors

Energy (scalar) vs momentum (vector); confusion leads to sign errors.

### ❌ Wrong reference frame

Inertial vs non-inertial. Fictitious forces in rotating frames.

## Failure modes

| Failure | Recovery |
|---------|----------|
| Numerical instability | Smaller step; implicit method; better conditioning |
| Dimensional mismatch | Re-derive with explicit units |
| Model fails at extreme | Limiting-case analysis; regime identification |
| Wrong governing equation | Re-derive from first principles |
| Numerical drift | Symplectic integrator; constraint preservation |

## Related skills

| Skill | When to chain |
|-------|---------------|
| `/quantum-physics` | Quantum regime; atomic scale |
| `/chemistry` | Molecular modelling; reactions |
| `/biology` | Biomechanics; biophysics |
| `/engineering` | Engineering applications |

## Citations

- Taylor, "Classical Mechanics"
- Griffiths, "Introduction to Electrodynamics"
- Landau & Lifshitz, "Statistical Physics"
- Feynman Lectures on Physics