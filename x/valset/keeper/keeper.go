package keeper

import (
	"fmt"

	"github.com/cosmos/cosmos-sdk/crypto/keys/ed25519"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	"github.com/cosmos/cosmos-sdk/store/prefix"
	"github.com/tendermint/tendermint/libs/log"

	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	paramtypes "github.com/cosmos/cosmos-sdk/x/params/types"
	keeperutil "github.com/volumefi/cronchain/util/keeper"
	"github.com/volumefi/cronchain/x/valset/types"
)

type (
	Keeper struct {
		cdc        codec.BinaryCodec
		storeKey   sdk.StoreKey
		memKey     sdk.StoreKey
		paramstore paramtypes.Subspace
		staking    types.StakingKeeper
	}
)

func NewKeeper(
	cdc codec.BinaryCodec,
	storeKey,
	memKey sdk.StoreKey,
	ps paramtypes.Subspace,
	staking types.StakingKeeper,

) *Keeper {
	// set KeyTable if it has not already been set
	if !ps.HasKeyTable() {
		ps = ps.WithKeyTable(types.ParamKeyTable())
	}

	return &Keeper{
		cdc:        cdc,
		storeKey:   storeKey,
		memKey:     memKey,
		paramstore: ps,
		staking:    staking,
	}
}

func (k Keeper) Logger(ctx sdk.Context) log.Logger {
	return ctx.Logger().With("module", fmt.Sprintf("x/%s", types.ModuleName))
}

// TODO: not required now
func (k Keeper) PunishValidator(ctx sdk.Context) {}

// TODO: not required now
func (k Keeper) Heartbeat(ctx sdk.Context) {}

// TODO: not required now
// TODO: break this into add, remove
func (k Keeper) updateExternalChainInfo(ctx sdk.Context) {}

// Register registers the validator as being a part of a conductor's network.
func (k Keeper) Register(ctx sdk.Context, msg *types.MsgRegisterConductor) error {

	valAddr, err := sdk.ValAddressFromBech32(msg.Creator)
	if err != nil {
		return err
	}

	sval := k.staking.Validator(ctx, valAddr)
	if sval == nil {
		return ErrValidatorWithAddrNotFound.Format(valAddr)
	}

	// TODO: making the assumption that the pub key is of ed25519 type.
	pk := &ed25519.PubKey{
		Key: msg.PubKey,
	}

	if !pk.VerifySignature(msg.PubKey, msg.SignedPubKey) {
		return ErrPublicKeyOrSignatureIsInvalid
	}

	store := k.validatorStore(ctx)

	// check if is already registered! if yes, then error
	if store.Has(valAddr) {
		return ErrValidatorAlreadyRegistered
	}

	val := &types.Validator{
		Address: sval.GetOperator().String(),
		// TODO: add the rest
	}

	// TODO: more logic here
	val.State = types.ValidatorState_ACTIVE

	// save val
	return keeperutil.Save(store, k.cdc, valAddr, val)
}

// CreateSnapshot creates the snapshot of currently active validators that are
// active and registered as conductors.
func (k Keeper) CreateSnapshot(ctx sdk.Context) error {
	// TODO: check if there is a need for snapshots being incremental and keeping the historical versions.
	valStore := k.validatorStore(ctx)

	// get all registered validators
	validators, err := keeperutil.IterAll[*types.Validator](valStore, k.cdc)
	if err != nil {
		return err
	}

	snapshot := &types.Snapshot{
		Height:      ctx.BlockHeight(),
		CreatedAt:   ctx.BlockTime(),
		TotalShares: sdk.ZeroInt(),
	}

	for _, val := range validators {
		// if val.State != types.ValidatorState_ACTIVE {
		// 	continue
		// }
		snapshot.TotalShares = snapshot.TotalShares.Add(val.ShareCount)
		snapshot.Validators = append(snapshot.Validators, *val)
	}

	return k.setSnapshotAsCurrent(ctx, snapshot)
}

func (k Keeper) setSnapshotAsCurrent(ctx sdk.Context, snapshot *types.Snapshot) error {
	snapStore := k.snapshotStore(ctx)
	return keeperutil.Save(snapStore, k.cdc, []byte("snapshot"), snapshot)
}

// GetCurrentSnapshot returns the currently active snapshot.
func (k Keeper) GetCurrentSnapshot(ctx sdk.Context) (*types.Snapshot, error) {
	snapStore := k.snapshotStore(ctx)
	return keeperutil.Load[*types.Snapshot](snapStore, k.cdc, []byte("snapshot"))
}

// GetSigningKey returns a signing key used by the conductor to sign arbitrary messages.
func (k Keeper) GetSigningKey(ctx sdk.Context, valAddr sdk.ValAddress) cryptotypes.PubKey {
	val := k.staking.Validator(ctx, valAddr)
	if val == nil {
		return nil
	}
	pk, _ := val.ConsPubKey()
	return pk
}

func (k Keeper) validatorStore(ctx sdk.Context) sdk.KVStore {
	return prefix.NewStore(ctx.KVStore(k.storeKey), []byte("validators"))
}

func (k Keeper) snapshotStore(ctx sdk.Context) sdk.KVStore {
	return prefix.NewStore(ctx.KVStore(k.storeKey), []byte("snapshot"))
}