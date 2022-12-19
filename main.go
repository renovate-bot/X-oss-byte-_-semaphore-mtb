package main

import (
	"encoding/json"
	"fmt"
	gnarkLogger "github.com/consensys/gnark/logger"
	"github.com/urfave/cli/v2"
	"io"
	"math/big"
	"os"
	"worldcoin/gnark-mbu/logging"
	"worldcoin/gnark-mbu/prover"
)

func main() {
	gnarkLogger.Set(*logging.Logger())
	app := cli.App{
		EnableBashCompletion: true,
		Commands: []*cli.Command{
			{
				Name: "setup",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "output", Usage: "Output file", Required: true},
					&cli.UintFlag{Name: "tree-depth", Usage: "Merkle tree depth", Required: true},
					&cli.UintFlag{Name: "batch-size", Usage: "Batch size", Required: true},
				},
				Action: func(context *cli.Context) error {
					path := context.String("output")
					treeDepth := uint32(context.Uint("tree-depth"))
					batchSize := uint32(context.Uint("batch-size"))
					logging.Logger().Info().Msg("Running setup")
					system, err := prover.Setup(treeDepth, batchSize)
					if err != nil {
						return err
					}
					file, err := os.Create(path)
					defer file.Close()
					if err != nil {
						return err
					}
					written, err := system.WriteTo(file)
					if err != nil {
						return err
					}
					logging.Logger().Info().Int64("bytesWritten", written).Msg("proving system written to file")
					return nil
				},
			},
			{
				Name: "export-solidity",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "keys-file", Usage: "proving system file", Required: true},
					&cli.StringFlag{Name: "output", Usage: "solidity output (will write to stdout if not provided)", Required: false},
				},
				Action: func(context *cli.Context) error {
					keys := context.String("keys-file")
					ps, err := prover.ReadSystemFromFile(keys)
					if err != nil {
						return err
					}
					var output io.Writer
					if outPath := context.String("output"); outPath != "" {
						file, err := os.Create(outPath)
						defer file.Close()
						if err != nil {
							return err
						}
						output = file
					} else {
						output = os.Stdout
					}
					return ps.ExportSolidity(output)
				},
			},
			{
				Name: "gen-test-params",
				Flags: []cli.Flag{
					&cli.UintFlag{Name: "tree-depth", Usage: "depth of the mock tree", Required: true},
					&cli.UintFlag{Name: "batch-size", Usage: "batch size", Required: true},
				},
				Action: func(context *cli.Context) error {
					treeDepth := context.Int("tree-depth")
					batchSize := uint32(context.Uint("batch-size"))
					logging.Logger().Info().Msg("Generating test params")

					params := prover.Parameters{}
					tree := NewTree(treeDepth)

					params.StartIndex = 0
					params.PreRoot = tree.Root()
					params.IdComms = make([]big.Int, batchSize)
					params.MerkleProofs = make([][]big.Int, batchSize)
					for i := 0; i < int(batchSize); i++ {
						params.IdComms[i] = *new(big.Int).SetUint64(uint64(i))
						params.MerkleProofs[i] = tree.Update(i, params.IdComms[i])
					}
					params.PostRoot = tree.Root()
					params.ComputeInputHash()
					r, _ := json.Marshal(&params)
					fmt.Println(string(r))
					return nil
				},
			},
			{
				Name: "prove",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "keys-file", Usage: "proving system file", Required: true},
				},
				Action: func(context *cli.Context) error {
					keys := context.String("keys-file")
					ps, err := prover.ReadSystemFromFile(keys)
					if err != nil {
						return err
					}
					logging.Logger().Info().Uint32("treeDepth", ps.TreeDepth).Uint32("batchSize", ps.BatchSize).Msg("Read proving system")
					logging.Logger().Info().Msg("reading params from stdin")
					bytes, err := io.ReadAll(os.Stdin)
					if err != nil {
						return err
					}
					var params prover.Parameters
					err = json.Unmarshal(bytes, &params)
					if err != nil {
						return err
					}
					logging.Logger().Info().Msg("params read successfully")
					proof, err := ps.Prove(&params)
					if err != nil {
						return err
					}
					r, _ := json.Marshal(&proof)
					fmt.Println(string(r))
					return nil
				},
			},
			{
				Name: "verify",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "keys-file", Usage: "proving system file", Required: true},
					&cli.StringFlag{Name: "input-hash", Usage: "the hash of all public inputs", Required: true},
				},
				Action: func(context *cli.Context) error {
					keys := context.String("keys-file")
					var inputHash big.Int
					_, ok := inputHash.SetString(context.String("input-hash"), 0)
					if !ok {
						return fmt.Errorf("invalid number: %s", context.String("input-hash"))
					}
					ps, err := prover.ReadSystemFromFile(keys)
					if err != nil {
						return err
					}
					logging.Logger().Info().Uint32("treeDepth", ps.TreeDepth).Uint32("batchSize", ps.BatchSize).Msg("Read proving system")
					logging.Logger().Info().Msg("reading proof from stdin")
					bytes, err := io.ReadAll(os.Stdin)
					if err != nil {
						return err
					}
					var proof prover.Proof
					err = json.Unmarshal(bytes, &proof)
					if err != nil {
						return err
					}
					logging.Logger().Info().Msg("proof read successfully")
					err = ps.Verify(inputHash, &proof)
					if err != nil {
						return err
					}
					logging.Logger().Info().Msg("verification complete")
					return nil
				},
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		logging.Logger().Fatal().Err(err).Msg("App failed.")
	}
}
