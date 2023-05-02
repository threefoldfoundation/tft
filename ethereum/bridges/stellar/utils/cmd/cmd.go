package cmd

import (
	"errors"
	"fmt"
	"os"
	"strconv"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	Secret string

	rootCmd = &cobra.Command{Use: "stellar-utils", Short: "A tool for bootstrapping Stellar bridges"}

	generateCmd = &cobra.Command{
		Use: "generate [bridge, account]",
	}

	generatePlainAccountCmd = &cobra.Command{
		Use:   "plain",
		Short: "generate plain account with XLM and TFT",
		Long:  "generate a new stellar account with XLM and TFT balances initialised",
		RunE: func(cmd *cobra.Command, args []string) error {
			kp, err := GenerateAccount()
			if err != nil {
				return err
			}

			fmt.Printf("\nNew Account address: %s\n", kp.Address())
			fmt.Printf("New Account secret: %s\n", kp.Seed())
			return nil
		},
	}

	generateBridgeAccountCmd = &cobra.Command{
		Use:     "bridge [count: int]",
		Short:   "generate bridge accounts [count: int]",
		Long:    "generate and activate account for the bridge",
		Example: "generate 2",
		Args:    cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
		RunE: func(cmd *cobra.Command, args []string) error {
			numberOfAccounts, err := strconv.Atoi(args[0])
			if err != nil {
				return err
			}

			if numberOfAccounts < 2 || numberOfAccounts > 20 {
				return errors.New("must provide a valid number of account > 2 && < 20")
			}
			fmt.Printf("generating %d accounts and setting account options ... \n", numberOfAccounts)
			accounts, err := GenerateAndActivateAccounts(numberOfAccounts)
			if err != nil {
				return err
			}

			err = SetAccountOptions(accounts)
			if err != nil {
				return err
			}

			fmt.Printf("\nBridge master address: %s\n", accounts[0].Address())
			fmt.Printf("Bridge master secret: %s\n", accounts[0].Seed())

			for i := 1; i < len(accounts); i++ {
				fmt.Println()
				fmt.Printf("Signer %d address: %s\n", i, accounts[i].Address())
				fmt.Printf("Signer %d secret: %s\n", i, accounts[i].Seed())
			}
			return nil
		},
	}

	transferCmd = &cobra.Command{
		Use:   "transfer [destinationAddress, amount, memo]",
		Short: "transfer TFT [destinationAddress, amount, memo]",
		Long:  "transfer from a specified account to another account given a private key",
		Args:  cobra.MatchAll(cobra.ExactArgs(3), cobra.OnlyValidArgs),
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println(args)

			// amount, err := strconv.ParseInt(args[1], 10, 64)
			// if err != nil {
			// 	return err
			// }

			err := Transfer(Secret, args[0], args[2], args[1])
			if err != nil {
				return err
			}

			fmt.Printf("transfered %s to %s with memo %s", args[1], args[0], args[2])

			return nil
		},
	}

	faucetCmd = &cobra.Command{
		Use:   "faucet",
		Short: "request stellar testnet TFT to your account",
		RunE: func(cmd *cobra.Command, args []string) error {
			return GetTestnetTFT(Secret)
		},
	}
)

func Execute() {
	generateCmd.AddCommand(generateBridgeAccountCmd, generatePlainAccountCmd)

	transferCmd.Flags().StringVar(&Secret, "secret", "", "Stellar secret")
	viper.BindPFlag("secret", transferCmd.Flags().Lookup("secret"))
	transferCmd.MarkFlagRequired("secret")

	faucetCmd.SetArgs([]string{"destinationAddress", "amount", "memo"})
	faucetCmd.Flags().StringVar(&Secret, "secret", "", "Stellar secret")
	viper.BindPFlag("secret", faucetCmd.Flags().Lookup("secret"))
	faucetCmd.MarkFlagRequired("secret")

	rootCmd.AddCommand(generateCmd)
	rootCmd.AddCommand(transferCmd)
	rootCmd.AddCommand(faucetCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
