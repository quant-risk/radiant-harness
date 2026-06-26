# Skill: biology

> Genomics, transcriptomics, proteomics, systems biology,
> bioinformatics pipelines. Reproducibility is non-negotiable;
> biology without controls is anecdote.

## Decision tree

```
Biological data analysis
        │
        ▼
[Step 1] Define question + experimental design
        │
        ▼
[Step 2] Sample prep + QC
        │
        ▼
[Step 3] Choose omics platform + pipeline
        │
        ▼
[Step 4] Compute (alignment / quantification / variant calling)
        │
        ▼
[Step 5] QC of analysis output
        │
        ▼
[Step 6] Downstream analysis (annotation, statistics)
        │
        ▼
[Step 7] Validation + interpretation
```

## Workflow

### NGS (Next-Generation Sequencing)

**Library prep**:
- DNA-seq: fragmentation, adapter ligation, PCR
- RNA-seq: mRNA enrichment or rRNA depletion; cDNA synthesis
- scRNA-seq: cell isolation, barcoding
- ChIP-seq: crosslink, sonicate, IP
- ATAC-seq: transposase insertion

**Sequencing platforms**:
- Illumina (short read, high accuracy)
- PacBio (long read, high fidelity)
- Oxford Nanopore (ultra-long, real-time)
- 10x Genomics (single-cell)

**Pipeline** (DNA-seq example):
1. QC: FastQC, MultiQC
2. Trim: Trimmomatic, fastp
3. Align: BWA-MEM, Bowtie2
4. Sort/dedup: Picard, samtools
5. Variant call: GATK, DeepVariant
6. Annotate: VEP, SnpEff
7. QC: bedtools, mosdepth

### Variant calling

**Germline variants**: GATK HaplotypeCaller, DeepVariant
**Somatic variants**: Mutect2, VarScan, LoFreq
**Structural variants**: Manta, DELLY, GRIDSS
**CNVs**: GATK CNV, Control-FREEC

**Validation**: Sanger sequencing; orthogonal platform.

### RNA-seq analysis

```
1. QC (FastQC)
2. Trim (fastp)
3. Align (STAR / HISAT2)
4. Quantify (featureCounts / RSEM / Salmon)
5. Normalise (TPM / FPKM / DESeq2 size factors)
6. Differential expression (DESeq2 / edgeR / limma-voom)
7. Pathway enrichment (GSEA / clusterProfiler)
```

QC: % alignment, rRNA contamination, duplication, coverage.

### Single-cell analysis

```
1. Cellranger (10x) or equivalent
2. QC: per-cell (genes, UMI, % mito); filter
3. Normalise (log-normalise, SCTransform)
4. HVG selection
5. PCA → UMAP / t-SNE
6. Clustering (Leiden / Louvain)
7. Marker genes (per cluster)
8. Cell type annotation
9. Trajectory inference (Monocle, Slingshot)
```

Tools: Seurat (R), Scanpy (Python).

### Proteomics

**Mass spec workflow**:
1. Sample prep (digest, fractionate)
2. LC-MS/MS
3. Database search (Mascot, MaxQuant)
4. Quantification (label-free, TMT, SILAC)
5. Statistics (limma, MSstats)

**Post-translational modifications**: phosphorylation,
ubiquitination, glycosylation.

### Protein structure

| Method | Use |
|--------|-----|
| **X-ray crystallography** | High resolution; requires crystal |
| **Cryo-EM** | Large complexes; no crystal needed |
| **NMR** | Solution; small proteins |
| **AlphaFold2 / RoseTTAFold** | ML-based prediction |
| **MD simulation** | Dynamics, not static structure |

### Systems biology

**Network analysis**:
- PPI networks (STRING, BioGRID)
- Signalling networks (KEGG, Reactome)
- Gene regulatory networks (GRN inference)

**ODE models**:
- Michaelis-Menten kinetics
- Mass action kinetics
- Stochastic (Gillespie algorithm)

**Constraint-based**:
- FBA (flux balance analysis)
- FVA (flux variability analysis)

### CRISPR / gene editing

**Design**: sgRNA (Cas9); PAM sequence; off-target prediction
(CRISPOR, CHOPCHOP).

**Validation**: amplicon sequencing; TIDE / ICE analysis.

**Applications**: knock-out, knock-in, activation, repression
(dCas9 fusions).

### Drug discovery (biology side)

1. Target identification (genetic, genomic, literature)
2. Hit identification (HTS, virtual screening)
3. Lead optimisation (SAR, ADMET)
4. Preclinical (in vitro, in vivo, tox)
5. Clinical trials

## Examples

### Example 1: variant calling (germline)

```
Sample: NA12878 (control)
Pipeline: BWA-MEM → Picard dedup → GATK HaplotypeCaller → VEP
Result: 4.2M variants (3.9M SNVs, 0.3M indels)
Validation: vs GIAB truth set
  - SNV precision: 99.7%
  - SNV recall: 99.4%
  - Indel F1: 96.8%
```

### Example 2: differential expression (RNA-seq)

```
Design: 6 tumour vs 6 normal (paired)
Pipeline: STAR → featureCounts → DESeq2
Result: 1,847 DE genes (FDR < 0.05, |log2FC| > 1)
Top pathways (GSEA):
  - Cell cycle (p < 0.001)
  - p53 signalling (p < 0.001)
  - DNA repair (p < 0.001)
Validation: qPCR for top 10 (9/10 concordant)
```

### Example 3: single-cell clustering

```
Data: 10k PBMCs (10x)
QC: 200-5000 genes/cell, <10% mito
Normalise: log-normalise
HVG: 2000
PCA: 50 components → UMAP
Clustering: Leiden (resolution 0.5)
Result: 8 clusters
Annotation: T cell (3), B cell (2), NK (1), mono (2)
Markers: CD3D (T), CD19 (B), NKG7 (NK), CD14 (mono)
```

## Anti-patterns

### ❌ No QC before analysis

Garbage in, garbage out. Always QC raw + processed.

### ❌ Non-reproducible pipeline

Containerise (Docker / Singularity / conda env).

### ❌ No validation samples

Variants / DE genes may be wrong. Always include controls.

### ❌ Ignoring batch effects

Confounders masquerade as biology. Combat, limma, etc.

### ❌ Multiple testing without correction

20k genes × 0.05 = 1000 false positives. Use FDR (BH).

### ❌ Cross-species without orthology mapping

Mouse → human ≠ direct. Use orthologs.

## Failure modes

| Failure | Recovery |
|---------|----------|
| Low alignment rate | Adapter contamination; rRNA; poor quality |
| Batch effects | ComBat; design matrix; balanced design |
| Variant calls wrong | Validate with orthogonal method; Sanger |
| DE results not reproducible | More samples; better power |
| scRNA-seq batch | Harmony, scVI integration |
| Structure wrong | Multiple methods; experimental validation |

## Related skills

| Skill | When to chain |
|-------|---------------|
| `/chemistry` | Molecular interactions; drug discovery |
| `/stats` | Multiple testing; statistical inference |
| `/ml` | ML for biology (AlphaFold, scVI) |
| `/quantum-ml` | Quantum chemistry for proteins |

## Tools

| Tool | Purpose |
|------|---------|
| **samtools / bcftools** | SAM/BAM/VCF manipulation |
| **GATK** | Variant calling |
| **STAR / HISAT2** | RNA-seq alignment |
| **DESeq2 / edgeR** | Differential expression |
| **Seurat / Scanpy** | Single-cell analysis |
| **Snakemake / Nextflow** | Pipeline management |
| **AlphaFold2 / RoseTTAFold** | Structure prediction |

## Citations

- Goodwin et al., "Coming of age: ten years of next-generation sequencing"
- Love et al., "Moderated estimation of fold change", Genome Biology 2014
- Stuart et al., "Comprehensive Integration of Single-Cell Data", Cell 2019
- Jumper et al., "Highly accurate protein structure prediction with AlphaFold", Nature 2021