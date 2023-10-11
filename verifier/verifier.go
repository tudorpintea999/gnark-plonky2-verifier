package verifier

import (
	"github.com/consensys/gnark/frontend"
	"github.com/succinctlabs/gnark-plonky2-verifier/challenger"
	"github.com/succinctlabs/gnark-plonky2-verifier/fri"
	gl "github.com/succinctlabs/gnark-plonky2-verifier/goldilocks"
	"github.com/succinctlabs/gnark-plonky2-verifier/plonk"
	"github.com/succinctlabs/gnark-plonky2-verifier/poseidon"
	"github.com/succinctlabs/gnark-plonky2-verifier/types"
)

type VerifierChip struct {
	api               frontend.API             `gnark:"-"`
	glChip            *gl.GoldilocksApi        `gnark:"-"`
	poseidonGlChip    *poseidon.GoldilocksChip `gnark:"-"`
	poseidonBN254Chip *poseidon.BN254Chip      `gnark:"-"`
	plonkChip         *plonk.PlonkChip         `gnark:"-"`
	friChip           *fri.Chip                `gnark:"-"`
}

func NewVerifierChip(api frontend.API, commonCircuitData types.CommonCircuitData) *VerifierChip {
	glChip := gl.NewGoldilocksApi(api)
	friChip := fri.NewChip(api, &commonCircuitData.FriParams)
	plonkChip := plonk.NewPlonkChip(api, commonCircuitData)
	poseidonGlChip := poseidon.NewGoldilocksChip(api)
	poseidonBN254Chip := poseidon.NewBN254Chip(api)
	return &VerifierChip{
		api:               api,
		glChip:            glChip,
		poseidonGlChip:    poseidonGlChip,
		poseidonBN254Chip: poseidonBN254Chip,
		plonkChip:         plonkChip,
		friChip:           friChip,
	}
}

func (c *VerifierChip) GetPublicInputsHash(publicInputs []gl.GoldilocksVariable) poseidon.GoldilocksHashOut {
	return c.poseidonGlChip.HashNoPad(publicInputs)
}

func (c *VerifierChip) GetChallenges(
	proof types.Proof,
	publicInputsHash poseidon.GoldilocksHashOut,
	commonData types.CommonCircuitData,
	verifierData types.VerifierOnlyCircuitData,
) types.ProofChallenges {
	config := commonData.Config
	numChallenges := config.NumChallenges
	challenger := challenger.NewChip(c.api)

	var circuitDigest = verifierData.CircuitDigest

	challenger.ObserveBN254Hash(circuitDigest)
	challenger.ObserveHash(publicInputsHash)
	challenger.ObserveCap(proof.WiresCap)
	plonkBetas := challenger.GetNChallenges(numChallenges)
	plonkGammas := challenger.GetNChallenges(numChallenges)

	challenger.ObserveCap(proof.PlonkZsPartialProductsCap)
	plonkAlphas := challenger.GetNChallenges(numChallenges)

	challenger.ObserveCap(proof.QuotientPolysCap)
	plonkZeta := challenger.GetExtensionChallenge()

	challenger.ObserveOpenings(fri.ToOpenings(proof.Openings))

	return types.ProofChallenges{
		PlonkBetas:  plonkBetas,
		PlonkGammas: plonkGammas,
		PlonkAlphas: plonkAlphas,
		PlonkZeta:   plonkZeta,
		FriChallenges: challenger.GetFriChallenges(
			proof.OpeningProof.CommitPhaseMerkleCaps,
			proof.OpeningProof.FinalPoly,
			proof.OpeningProof.PowWitness,
			commonData.DegreeBits,
			config.FriConfig,
		),
	}
}

/*
func (c *VerifierChip) generateProofInput(commonData common.CommonCircuitData) common.ProofWithPublicInputs {
	// Generate the parts of the witness that is for the plonky2 proof input

	capHeight := commonData.Config.FriConfig.CapHeight

	friCommitPhaseMerkleCaps := []common.MerkleCap{}
	for i := 0; i < len(commonData.FriParams.ReductionArityBits); i++ {
		friCommitPhaseMerkleCaps = append(friCommitPhaseMerkleCaps, common.NewMerkleCap(capHeight))
	}

	salt := commonData.SaltSize()
	numLeavesPerOracle := []uint{
		commonData.NumPreprocessedPolys(),
		commonData.Config.NumWires + salt,
		commonData.NumZsPartialProductsPolys() + salt,
		commonData.NumQuotientPolys() + salt,
	}
	friQueryRoundProofs := []common.FriQueryRound{}
	for i := uint64(0); i < commonData.FriParams.Config.NumQueryRounds; i++ {
		evalProofs := []common.EvalProof{}
		merkleProofLen := commonData.FriParams.LDEBits() - capHeight
		for _, numLeaves := range numLeavesPerOracle {
			leaves := make([]field.F, numLeaves)
			merkleProof := common.NewMerkleProof(merkleProofLen)
			evalProofs = append(evalProofs, common.NewEvalProof(leaves, merkleProof))
		}

		initialTreesProof := common.NewFriInitialTreeProof(evalProofs)
		steps := []common.FriQueryStep{}
		for _, arityBit := range commonData.FriParams.ReductionArityBits {
			if merkleProofLen < arityBit {
				panic("merkleProofLen < arityBits")
			}

			steps = append(steps, common.NewFriQueryStep(arityBit, merkleProofLen))
		}

		friQueryRoundProofs = append(friQueryRoundProofs, common.NewFriQueryRound(steps, initialTreesProof))
	}

	proofInput := common.ProofWithPublicInputs{
		Proof: common.Proof{
			WiresCap:                  common.NewMerkleCap(capHeight),
			PlonkZsPartialProductsCap: common.NewMerkleCap(capHeight),
			QuotientPolysCap:          common.NewMerkleCap(capHeight),
			Openings: common.NewOpeningSet(
				commonData.Config.NumConstants,
				commonData.Config.NumRoutedWires,
				commonData.Config.NumWires,
				commonData.Config.NumChallenges,
				commonData.NumPartialProducts,
				commonData.QuotientDegreeFactor,
			),
			OpeningProof: common.FriProof{
				CommitPhaseMerkleCaps: friCommitPhaseMerkleCaps,
				QueryRoundProofs:      friQueryRoundProofs,
				FinalPoly:             common.NewPolynomialCoeffs(commonData.FriParams.FinalPolyLen()),
			},
		},
		PublicInputs: make([]field.F, commonData.NumPublicInputs),
	}

	return proofInput
}
*/

func (c *VerifierChip) rangeCheckProof(proof types.Proof) {
	// Need to verify the plonky2 proof's openings, openings proof (other than the sibling elements), fri's final poly, pow witness.

	// Note that this is NOT range checking the public inputs (first 32 elements should be no more than 8 bits and the last 4 elements should be no more than 64 bits).  Since this is currently being inputted via the smart contract,
	// we will assume that caller is doing that check.

	// Range check the proof's openings.
	for _, constant := range proof.Openings.Constants {
		c.glChip.RangeCheckQE(constant)
	}

	for _, plonkSigma := range proof.Openings.PlonkSigmas {
		c.glChip.RangeCheckQE(plonkSigma)
	}

	for _, wire := range proof.Openings.Wires {
		c.glChip.RangeCheckQE(wire)
	}

	for _, plonkZ := range proof.Openings.PlonkZs {
		c.glChip.RangeCheckQE(plonkZ)
	}

	for _, plonkZNext := range proof.Openings.PlonkZsNext {
		c.glChip.RangeCheckQE(plonkZNext)
	}

	for _, partialProduct := range proof.Openings.PartialProducts {
		c.glChip.RangeCheckQE(partialProduct)
	}

	for _, quotientPoly := range proof.Openings.QuotientPolys {
		c.glChip.RangeCheckQE(quotientPoly)
	}

	// Range check the openings proof.
	for _, queryRound := range proof.OpeningProof.QueryRoundProofs {
		for _, initialTreesElement := range queryRound.InitialTreesProof.EvalsProofs[0].Elements {
			c.glChip.RangeCheck(initialTreesElement)
		}

		for _, queryStep := range queryRound.Steps {
			for _, eval := range queryStep.Evals {
				c.glChip.RangeCheckQE(eval)
			}
		}
	}

	// Range check the fri's final poly.
	for _, coeff := range proof.OpeningProof.FinalPoly.Coeffs {
		c.glChip.RangeCheckQE(coeff)
	}

	// Range check the pow witness.
	c.glChip.RangeCheck(proof.OpeningProof.PowWitness)
}

func (c *VerifierChip) Verify(
	proof types.Proof,
	publicInputs []gl.GoldilocksVariable,
	verifierData types.VerifierOnlyCircuitData,
	commonData types.CommonCircuitData,
) {
	c.rangeCheckProof(proof)

	// Generate the parts of the witness that is for the plonky2 proof input
	publicInputsHash := c.GetPublicInputsHash(publicInputs)
	proofChallenges := c.GetChallenges(proof, publicInputsHash, commonData, verifierData)

	c.plonkChip.Verify(proofChallenges, proof.Openings, publicInputsHash)

	initialMerkleCaps := []types.FriMerkleCap{
		verifierData.ConstantSigmasCap,
		proof.WiresCap,
		proof.PlonkZsPartialProductsCap,
		proof.QuotientPolysCap,
	}

	c.friChip.VerifyFriProof(
		fri.GetInstance(&commonData, c.glChip, proofChallenges.PlonkZeta, commonData.DegreeBits),
		fri.ToOpenings(proof.Openings),
		&proofChallenges.FriChallenges,
		initialMerkleCaps,
		&proof.OpeningProof,
	)
}
