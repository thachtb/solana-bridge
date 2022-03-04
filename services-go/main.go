package main

import (
	"context"
	"fmt"
	"github.com/davecgh/go-spew/spew"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/gagliardetto/solana-go/rpc/ws"
	"github.com/thachtb/solana-bridge/services-go/Shield"
	"strings"
)

const SHIELD = "Shield"
const UNSHIELD = "Unshield"
const INCOGNITO_PROXY = "8WUP1RGTDTZGYBjkHQfjnwMbnnk25hnE6Du7vFpaq1QK"
const PROGRAM_ID = "BKGhwbiTHdUxcuWzZtDWyioRBieDEXTtgEk8u1zskZnk"

func main() {
	// init vars
	// Create a new WS client (used for confirming transactions)
	wsClient, err := ws.Connect(context.Background(), rpc.DevNet_WS)
	if err != nil {
		panic(err)
	}

	program := solana.MustPublicKeyFromBase58(PROGRAM_ID)
	incognitoProxy := solana.MustPublicKeyFromBase58(INCOGNITO_PROXY)
	feePayer, err := solana.PrivateKeyFromBase58("588FU4PktJWfGfxtzpAAXywSNt74AvtroVzGfKkVN1LwRuvHwKGr851uH8czM5qm4iqLbs1kKoMKtMJG4ATR7Ld2")
	if err != nil {
		panic(err)
	}
	shieldMaker, err := solana.PrivateKeyFromBase58("28BD5MCpihGHD3zUfPv4VcBizis9zxFdaj9fGJmiyyLmezT94FRd9XiiLjz5gasgyX3TmH1BU4scdVE6gzDFzrc7")
	if err != nil {
		panic(err)
	}
	// Create a new RPC client:
	rpcClient := rpc.New(rpc.DevNet_RPC)

	// test shield tx
	recent, err := rpcClient.GetRecentBlockhash(context.Background(), rpc.CommitmentFinalized)
	if err != nil {
		panic(err)
	}

	fmt.Println("============ TEST SHIELD =============")

	shieldMakerTokenAccount := solana.MustPublicKeyFromBase58("5397KrEBCuEhdTjWF5B9xjVzGJR6MyxXLP3srbrWo2gD")
	vaultTokenAcc := solana.MustPublicKeyFromBase58("6dvNfGjtaErEefhUkDJtPhsxKxCxVDCMuVvyEdWsEgQu")

	incAddress := "12shR6fDe7ZcprYn6rjLwiLcL7oJRiek66ozzYu3B3rBxYXkqJeZYj6ZWeYy4qR4UHgaztdGYQ9TgHEueRXN7VExNRGB5t4auo3jTgXVBiLJmnTL5LzqmTXezhwmQvyrRjCbED5xW7yMMeeWarKa"
	shieldAmount := uint64(100000)
	shieldAccounts := []*solana.AccountMeta{
		solana.NewAccountMeta(shieldMakerTokenAccount, true, false),
		solana.NewAccountMeta(vaultTokenAcc, true, false),
		solana.NewAccountMeta(incognitoProxy, false, false),
		solana.NewAccountMeta(shieldMaker.PublicKey(), false, true),
		solana.NewAccountMeta(solana.TokenProgramID, false, false),
	}
	signers := []solana.PrivateKey{
		feePayer,
		shieldMaker,
	}

	shieldInstruction := shield.NewShield(
			incAddress,
			shieldAmount,
			program,
			shieldAccounts,
		)
	shieldInsGenesis := shieldInstruction.Build()
	if shieldInsGenesis == nil {
		panic("Build inst got error")
	}
	//amount := uint64(1000)
	tx, err := solana.NewTransaction(
		[]solana.Instruction{
			shieldInsGenesis,
		},
		recent.Value.Blockhash,
		solana.TransactionPayer(feePayer.PublicKey()),
	)
	if err != nil {
		panic(err)
	}
	sig, err := SignAndSendTx(tx, signers, rpcClient)
	if err != nil {
		panic(err)
	}
	spew.Dump(sig)

	fmt.Println("============ TEST UNSHIELD =============")


	fmt.Println("============ TEST LISTEN SHIELD EVENT =============")
	// listen shield to vault logs
	{
		// Subscribe to log events that mention the provided pubkey:
		sub, err := wsClient.LogsSubscribeMentions(
			program,
			rpc.CommitmentFinalized,
		)
		if err != nil {
			panic(err)
		}
		defer sub.Unsubscribe()

		for {
			got, err := sub.Recv()
			if err != nil {
				panic(err)
			}
			// dump to struct { signature , error, value }
			spew.Dump(got)
			processShield(got)
		}
	}
}

func processShield(logs *ws.LogResult) {
	if logs.Value.Err != nil {
		fmt.Printf("the transaction failed %v \n", logs.Value.Err)
		return
	}

	if len(logs.Value.Logs) < 7 {
		fmt.Printf("invalid shield logs, length must greate than 7 %v \n", logs.Value.Err)
		return
	}
	// todo: check signature and store if new
	//logs.Value.Signature

	shieldLogs := logs.Value.Logs
	// check shield instruction
	if !strings.Contains(shieldLogs[1], SHIELD) {
		fmt.Printf("invalid instruction %s\n", shieldLogs[1])
		return
	}

	shieldInfoSplit := strings.Split(shieldLogs[6], ":")
	if len(shieldInfoSplit) < 3 {
		fmt.Printf("invalid shield logs %+v\n", logs)
		return
	}

	shieldInfo := strings.Split(shieldInfoSplit[2], ",")
	if len(shieldInfo) < 4 {
		fmt.Printf("invalid shield info %v\n", shieldInfo)
		return
	}

	incognitoProxy := shieldInfo[0]
	if incognitoProxy != INCOGNITO_PROXY {
		fmt.Printf("invalid incognito proxy %v \n", incognitoProxy)
		return
	}
	incAddress := shieldInfo[1]
	tokenID := shieldInfo[2]
	amount := shieldInfo[3]

	fmt.Printf("shield with inc address %s token id %s and amount %s \n", incAddress, tokenID, amount)
}

func SignAndSendTx(tx *solana.Transaction, signers []solana.PrivateKey, rpcClient *rpc.Client) (solana.Signature, error) {
	_, err := tx.Sign(func(key solana.PublicKey) *solana.PrivateKey {
		for _, candidate := range signers {
			if candidate.PublicKey().Equals(key) {
				return &candidate
			}
		}
		return nil
	})
	if err != nil {
		fmt.Printf("unable to sign transaction: %v \n", err)
		return solana.Signature{}, err
	}
	// send tx
	signature, err := rpcClient.SendTransaction(context.Background(), tx)
	if err != nil {
		fmt.Printf("unable to send transaction: %v \n", err)
		return solana.Signature{}, err
	}
	return signature, nil
}