// Copyright 2018 The go-ethereum Authors
// This file is part of CortexFoundation.
//
// CortexFoundation is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// CortexFoundation is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with CortexFoundation. If not, see <http://www.gnu.org/licenses/>.

package main

import (
	"encoding/hex"
	"fmt"
	"os"

	"gopkg.in/urfave/cli.v1"

	"github.com/CortexFoundation/CortexTheseus/accounts/keystore"
	"github.com/CortexFoundation/CortexTheseus/cmd/utils"
	"github.com/CortexFoundation/CortexTheseus/common"
	"github.com/CortexFoundation/CortexTheseus/crypto"
)

type outputSign struct {
	Signature string
}

var msgfileFlag = cli.StringFlag{
	Name:  "msgfile",
	Usage: "file containing the message to sign/verify",
}

var commandSignMessage = cli.Command{
	Name:      "signmessage",
	Usage:     "sign a message",
	ArgsUsage: "<keyfile> <message>",
	Description: `
Sign the message with a keyfile.

To sign a message contained in a file, use the --msgfile flag.
`,
	Flags: []cli.Flag{
		passphraseFlag,
		jsonFlag,
		msgfileFlag,
	},
	Action: func(ctx *cli.Context) error {
		message := getMessage(ctx, 1)

		// Load the keyfile.
		keyfilepath := ctx.Args().First()
		keyjson, err := os.ReadFile(keyfilepath)
		if err != nil {
			utils.Fatalf("Failed to read the keyfile at '%s': %v", keyfilepath, err)
		}

		// Decrypt key with passphrase.
		passphrase := getPassphrase(ctx)
		key, err := keystore.DecryptKey(keyjson, passphrase)
		if err != nil {
			utils.Fatalf("Error decrypting key: %v", err)
		}

		signature, err := crypto.Sign(signHash(message), key.PrivateKey)
		if err != nil {
			utils.Fatalf("Failed to sign message: %v", err)
		}
		out := outputSign{Signature: hex.EncodeToString(signature)}
		if ctx.Bool(jsonFlag.Name) {
			mustPrintJSON(out)
		} else {
			fmt.Println("Signature:", out.Signature)
		}
		return nil
	},
}

type outputVerify struct {
	Success            bool
	RecoveredAddress   string
	RecoveredPublicKey string
}

var commandVerifyMessage = cli.Command{
	Name:      "verifymessage",
	Usage:     "verify the signature of a signed message",
	ArgsUsage: "<address> <signature> <message>",
	Description: `
Verify the signature of the message.
It is possible to refer to a file containing the message.`,
	Flags: []cli.Flag{
		jsonFlag,
		msgfileFlag,
	},
	Action: func(ctx *cli.Context) error {
		addressStr := ctx.Args().First()
		signatureHex := ctx.Args().Get(1)
		message := getMessage(ctx, 2)

		if !common.IsHexAddress(addressStr) {
			utils.Fatalf("Invalid address: %s", addressStr)
		}
		address := common.HexToAddress(addressStr)
		signature, err := hex.DecodeString(signatureHex)
		if err != nil {
			utils.Fatalf("Signature encoding is not hexadecimal: %v", err)
		}

		recoveredPubkey, err := crypto.SigToPub(signHash(message), signature)
		if err != nil || recoveredPubkey == nil {
			utils.Fatalf("Signature verification failed: %v", err)
		}
		recoveredPubkeyBytes := crypto.FromECDSAPub(recoveredPubkey)
		recoveredAddress := crypto.PubkeyToAddress(*recoveredPubkey)
		success := address == recoveredAddress

		out := outputVerify{
			Success:            success,
			RecoveredPublicKey: hex.EncodeToString(recoveredPubkeyBytes),
			RecoveredAddress:   recoveredAddress.Hex(),
		}
		if ctx.Bool(jsonFlag.Name) {
			mustPrintJSON(out)
		} else {
			if out.Success {
				fmt.Println("Signature verification successful!")
			} else {
				fmt.Println("Signature verification failed!")
			}
			fmt.Println("Recovered public key:", out.RecoveredPublicKey)
			fmt.Println("Recovered address:", out.RecoveredAddress)
		}
		return nil
	},
}

func getMessage(ctx *cli.Context, msgarg int) []byte {
	if file := ctx.String("msgfile"); file != "" {
		if len(ctx.Args()) > msgarg {
			utils.Fatalf("Can't use --msgfile and message argument at the same time.")
		}
		msg, err := os.ReadFile(file)
		if err != nil {
			utils.Fatalf("Can't read message file: %v", err)
		}
		return msg
	} else if len(ctx.Args()) == msgarg+1 {
		return []byte(ctx.Args().Get(msgarg))
	}
	utils.Fatalf("Invalid number of arguments: want %d, got %d", msgarg+1, len(ctx.Args()))
	return nil
}
