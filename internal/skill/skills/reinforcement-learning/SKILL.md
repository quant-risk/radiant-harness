# Skill: reinforcement-learning

> MDPs, value/policy iteration, DQN, PPO, SAC, off-policy,
> multi-agent, sim-to-real. RL is high-variance; always
> multiple seeds, always a non-RL baseline.

## Decision tree

```
Sequential decision problem
        │
        ▼
[Step 1] Can you do supervised learning instead?
        │   - offline data + good features -> yes, skip RL
        │   - need exploration / long-term reward -> continue
        ▼
[Step 2] Define MDP (S, A, P, R, γ)
        │   - state space
        │   - action space (discrete / continuous)
        │   - transition dynamics (known? learned?)
        │   - reward function (sparse? shaped?)
        │   - discount factor
        ▼
[Step 3] Choose algorithm (value / policy / model-based)
        │
        ├── Tabular (small state-action) -> Q-learning / SARSA
        ├── Discrete actions -> DQN, QR-DQN, Rainbow
        ├── Continuous actions -> PPO, SAC, TD3
        ├── Sparse reward -> curiosity-driven, RIDE
        └── Offline only -> CQL, BCQ, IQL
        │
        ▼
[Step 4] Train + monitor (TensorBoard)
        │
        ▼
[Step 5] Evaluate (multiple seeds; deterministic eval)
        │
        ▼
[Step 6] Deploy (with safety wrappers)
```

## Workflow

### When to use RL

| Use RL when | Use supervised / heuristic when |
|-------------|-------------------------------|
| Need to explore | Have offline data |
| Long-term reward | One-step decision |
| Environment interactive | Static dataset |
| Policy must adapt | Offline policy suffices |
| Reward can't be specified | Reward = label |

**Don't use RL when supervised learning works.** RL is sample-
inefficient, high-variance, and hard to debug. Use it only when
you need exploration.

### MDP definition

```python
import gymnasium as gym

class MyEnv(gym.Env):
    def __init__(self):
        self.observation_space = gym.spaces.Box(...)
        self.action_space = gym.spaces.Discrete(n)

    def reset(self):
        ...

    def step(self, action):
        ...
        return obs, reward, terminated, truncated, info
```

The interface should follow `gymnasium` (formerly OpenAI Gym) or
`PettingZoo` (multi-agent).

### Reward shaping

| Issue | Symptom | Fix |
|-------|---------|-----|
| **Sparse reward** | Agent never finds signal | Reward shaping; curriculum; intrinsic motivation |
| **Reward hacking** | Agent exploits unintended | Tighten reward; adversarial design |
| **Reward misspecification** | Behaviour doesn't match intent | Iterative reward design; red-team |
| **Multi-objective** | Trade-offs unclear | Pareto; weighted sum; constrained |

Intrinsic motivation (when external reward sparse):
- **Curiosity** (ICM, RND): prediction error of next state
- **RIDE**: episodic + curiosity
- **Go-Explore**: return to rare states
- **NGU**: novelty

### Algorithm choice

| Algorithm | Type | Discrete | Continuous | Sample efficiency | Stability |
|-----------|------|----------|------------|-------------------|-----------|
| **Q-learning** | Value-based | ✓ | ✗ | low | tabular OK |
| **DQN** | Value | ✓ | ✗ | medium | good (target net, replay) |
| **REINFORCE** | Policy | ✓ | ✓ | very low | high variance |
| **A2C / A3C** | Actor-critic | ✓ | ✓ | medium | OK |
| **PPO** | Actor-critic | ✓ | ✓ | medium | very stable |
| **SAC** | Actor-critic | ✗ | ✓ | high | very stable |
| **TD3** | Actor-critic | ✗ | ✓ | high | stable |
| **DDPG** | Actor-critic | ✗ | ✓ | medium | less stable |
| **CQL** | Off-policy | ✓ | ✓ | n/a (offline) | conservative |
| **BCQ** | Off-policy | ✓ | ✓ | n/a (offline) | imitation-based |

### Networks for RL

| Observation | Network |
|-------------|---------|
| Low-dim vector | MLP (2-3 hidden layers, 64-256 units) |
| Image | CNN (NatureCNN, ImpalaCNN) |
| Text | Transformer (pretrained) |
| Graph | GNN |
| Multi-modal | Per-modality encoder + fusion |

### Exploration

| Strategy | When |
|----------|------|
| **ε-greedy** | Discrete; simple |
| **Boltzmann** | Discrete; soft |
| **Gaussian noise** | Continuous |
| **UCB** | Bandit-like |
| **Parameter noise** | Continuous; for DDPG/SAC |
| **Count-based** | Sparse rewards |
| **Curiosity** | Sparse rewards |

### Off-policy / offline RL

| Scenario | Algorithm |
|----------|-----------|
| Have logged data, no env access | CQL, IQL, BCQ |
| Limited env interaction | Conservative Q-learning |
| Distillation from expert | Behavioral cloning + RL |

**Offline RL warning**: easy to overfit; need strong regularisation
(conservative Q-values); validate on actual env interaction.

### Multi-agent

| Paradigm | When |
|----------|------|
| **Independent learners** | Simple; no coordination |
| **Centralised training, decentralised exec** (CTDE) | Coordination needed |
| **Self-play** | Two-player zero-sum |
| **Population-based** | Diverse opponents |

Tools: `PettingZoo`, `RLlib`, `MARLlib`.

### Sim-to-real

| Technique | When |
|-----------|------|
| **Domain randomisation** | Default |
| **Domain adaptation** | When sim gap measurable |
| **System identification** | Match sim params to real |
| **Progressive transfer** | Easy env first, then hard |
| **Real-world fine-tuning** | With safety |

## Examples

### Example 1: DQN on Atari

```
Env:    Pong-v5
Net:    CNN (NatureCNN)
Buffer: 1M transitions
Recipe:
  - Adam, lr=6.25e-5
  - ε-greedy: 1.0 → 0.1 over 1M steps
  - Target net update every 10k steps
  - Reward clip [-1, 1]
  - 10M steps; 5 seeds
Result: agent beats baseline (random)
```

### Example 2: PPO for continuous control

```
Env:    HalfCheetah-v4 (MuJoCo)
Net:    MLP (64, 64) actor + critic
Recipe:
  - Adam, lr=3e-4
  - GAE λ=0.95, γ=0.99
  - Clip ratio 0.2
  - 2048 steps / rollout, 10 epochs
  - 5 seeds
Result: ~6000 reward (SOTA ~8000)
```

### Example 3: offline RL (CQL)

```
Data:  logged transitions from behaviour policy (1M steps)
Eval:  separate test env
Recipe:
  - CQL with conservative penalty α=5
  - lr=3e-4
  - 100k gradient steps
Result: conservative policy beats behaviour policy by 18%
```

## Anti-patterns

### ❌ RL when supervised learning works

If you have offline data + good features, use supervised learning.
RL is expensive and unstable.

### ❌ Sparse reward without shaping

Agent never finds reward signal. Shaping or intrinsic motivation
needed.

### ❌ Single seed

RL has high variance. Always multiple seeds (>=3).

### ❌ Reward hacking

Agent finds unintended ways to maximise reward. Tighten reward;
red-team; adversarial design.

### ❌ Sim-to-real without domain randomisation

Brittle transfer. Randomise textures, lighting, dynamics in sim.

### ❌ No safety constraints

RL explores aggressively. State bounds, action limits, reward
penalties needed.

## Failure modes

| Failure | Recovery |
|---------|----------|
| Agent not learning | Check reward; simplify env; curriculum |
| High variance across seeds | More seeds; PPO instead of REINFORCE; ensemble |
| Reward hacking | Tighten reward; red-team |
| Sim-to-real gap | Domain randomisation; progressive transfer |
| Catastrophic forgetting | Replay buffer; periodic fine-tuning |
| Offline RL overfits | Conservative methods; small batch; validation |

## Related skills

| Skill | When to chain |
|-------|---------------|
| `/ml` | Project-level ML workflow |
| `/deep-learning` | Neural network architectures for RL |
| `/bayesian` | Thompson sampling; posterior over policies |
| `/game` | Game environments; simulation |