package cli

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/context"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	jsoniter "github.com/json-iterator/go"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/maticnetwork/heimdall/bor/types"
	hmClient "github.com/maticnetwork/heimdall/client"
	"github.com/maticnetwork/heimdall/helper"
	hmTypes "github.com/maticnetwork/heimdall/types"
)

var cliLogger = helper.Logger.With("module", "bor/client/cli")

// GetTxCmd returns the transaction commands for this module
func GetTxCmd(cdc *codec.Codec) *cobra.Command {
	txCmd := &cobra.Command{
		Use:                        types.ModuleName,
		Short:                      "Bor transaction subcommands",
		DisableFlagParsing:         true,
		SuggestionsMinimumDistance: 2,
		RunE:                       hmClient.ValidateCmd,
	}

	txCmd.AddCommand(
		client.PostCommands(
			PostSendProposeSpanTx(cdc),
		)...,
	)

	return txCmd
}

// PostSendProposeSpanTx send propose span transaction
func PostSendProposeSpanTx(cdc *codec.Codec) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "propose-span",
		Short: "send propose span tx",
		RunE: func(cmd *cobra.Command, args []string) error {
			cliCtx := context.NewCLIContext().WithCodec(cdc)
			borChainID := viper.GetString(FlagBorChainId)
			if borChainID == "" {
				return fmt.Errorf("BorChainID cannot be empty")
			}

			// get proposer
			proposer := hmTypes.HexToHeimdallAddress(viper.GetString(FlagProposerAddress))
			if proposer.Empty() {
				proposer = helper.GetFromAddress(cliCtx)
			}

			// start block

			startBlockStr := viper.GetString(FlagStartBlock)
			if startBlockStr == "" {
				return fmt.Errorf("Start block cannot be empty")
			}

			startBlock, err := strconv.ParseUint(startBlockStr, 10, 64)
			if err != nil {
				return err
			}

			// span

			spanIDStr := viper.GetString(FlagSpanId)
			if spanIDStr == "" {
				return fmt.Errorf("Span Id cannot be empty")
			}

			spanID, err := strconv.ParseUint(spanIDStr, 10, 64)
			if err != nil {
				return err
			}

			nodeStatus, err := helper.GetNodeStatus(cliCtx)
			if err != nil {
				return err
			}

			//
			// Query data
			//

			// fetch duration
			res, _, err := cliCtx.QueryWithData(fmt.Sprintf("custom/%s/%s/%s", types.QuerierRoute, types.QueryParams, types.ParamSpan), nil)
			if err != nil {
				return err
			}
			if len(res) == 0 {
				return errors.New("span duration not found")
			}

			var spanDuration uint64
			if err = jsoniter.ConfigFastest.Unmarshal(res, &spanDuration); err != nil {
				return err
			}

			seedQueryParams, err := cliCtx.Codec.MarshalJSON(types.NewQuerySpanParams(spanID))
			if err != nil {
				return err
			}

			res, _, err = cliCtx.QueryWithData(fmt.Sprintf("custom/%s/%s", types.QuerierRoute, types.QueryNextSpanSeed), seedQueryParams)
			if err != nil {
				return err
			}

			if len(res) == 0 {
				return errors.New("next span seed not found")
			}

			var seedResponse types.QuerySpanSeedResponse
			if err := jsoniter.ConfigFastest.Unmarshal(res, &seedResponse); err != nil {
				return err
			}

			var msg sdk.Msg
			if nodeStatus.SyncInfo.LatestBlockHeight < helper.GetDanelawHeight() {
				msg = types.NewMsgProposeSpan(
					spanID,
					proposer,
					startBlock,
					startBlock+spanDuration-1,
					borChainID,
					seedResponse.Seed,
				)
			} else {
				msg = types.NewMsgProposeSpanV2(
					spanID,
					proposer,
					startBlock,
					startBlock+spanDuration-1,
					borChainID,
					seedResponse.Seed,
					seedResponse.SeedAuthor,
				)
			}

			return helper.BroadcastMsgsWithCLI(cliCtx, []sdk.Msg{msg})
		},
	}

	cmd.Flags().StringP(FlagProposerAddress, "p", "", "--proposer=<proposer-address>")
	cmd.Flags().String(FlagSpanId, "", "--span-id=<span-id>")
	cmd.Flags().String(FlagBorChainId, "", "--bor-chain-id=<bor-chain-id>")
	cmd.Flags().String(FlagStartBlock, "", "--start-block=<start-block-number>")

	if err := cmd.MarkFlagRequired(FlagBorChainId); err != nil {
		cliLogger.Error("PostSendProposeSpanTx | MarkFlagRequired | FlagBorChainId", "Error", err)
	}

	if err := cmd.MarkFlagRequired(FlagStartBlock); err != nil {
		cliLogger.Error("PostSendProposeSpanTx | MarkFlagRequired | FlagStartBlock", "Error", err)
	}

	return cmd
}
