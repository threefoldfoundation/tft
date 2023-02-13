package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	numberOfAccounts int

	rootCmd = &cobra.Command{
		Use:   "generate",
		Short: "",
		Long:  "generate and activate account for the bridge",
		RunE: func(cmd *cobra.Command, args []string) error {
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
			fmt.Printf("Bridge master address: %s\n", accounts[0].Seed())

			for i := 1; i < len(accounts); i++ {
				fmt.Println()
				fmt.Printf("Other signer address: %s\n", accounts[i].Address())
				fmt.Printf("Other signer address: %s\n", accounts[i].Seed())
			}
			return nil
		},
	}
)

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().IntVar(&numberOfAccounts, "count", 2, "Number of bridge accounts to generate")
}
