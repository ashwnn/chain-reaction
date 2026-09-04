package benchmark

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
)

const maxContractBytes = 1 << 20

// CanonicalJSON emits compact JSON only for map-free, integer-only contracts.
// Contract structs use declared field order and slices have explicit ordering.
func CanonicalJSON(value any) ([]byte, error) {
	if err := rejectNonCanonical(reflect.ValueOf(value)); err != nil {
		return nil, err
	}
	body, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("marshal canonical JSON: %w", err)
	}
	return body, nil
}

func Digest(value any) (string, error) {
	body, err := CanonicalJSON(value)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(body)
	return hex.EncodeToString(digest[:]), nil
}

func rejectNonCanonical(value reflect.Value) error {
	if !value.IsValid() {
		return nil
	}
	if value.Kind() == reflect.Interface || value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return nil
		}
		return rejectNonCanonical(value.Elem())
	}
	switch value.Kind() {
	case reflect.Map:
		return fmt.Errorf("canonical JSON does not permit maps")
	case reflect.Float32, reflect.Float64:
		return fmt.Errorf("canonical JSON does not permit floating-point values")
	case reflect.Struct:
		for index := 0; index < value.NumField(); index++ {
			if value.Type().Field(index).PkgPath != "" {
				continue
			}
			if err := rejectNonCanonical(value.Field(index)); err != nil {
				return err
			}
		}
	case reflect.Slice, reflect.Array:
		for index := 0; index < value.Len(); index++ {
			if err := rejectNonCanonical(value.Index(index)); err != nil {
				return err
			}
		}
	}
	return nil
}

func decodeStrict(data []byte, destination any) error {
	if len(data) == 0 || len(data) > maxContractBytes {
		return fmt.Errorf("contract payload must be between 1 and %d bytes", maxContractBytes)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return fmt.Errorf("decode contract: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return fmt.Errorf("contract has trailing JSON value")
		}
		return fmt.Errorf("decode trailing contract value: %w", err)
	}
	return nil
}

func DecodeScenarioManifest(data []byte) (ScenarioManifest, error) {
	var value ScenarioManifest
	if err := decodeStrict(data, &value); err != nil {
		return ScenarioManifest{}, err
	}
	return value, value.Validate()
}

func DecodeOracleContract(data []byte) (OracleContract, error) {
	var value OracleContract
	if err := decodeStrict(data, &value); err != nil {
		return OracleContract{}, err
	}
	return value, value.Validate()
}

func DecodeCommitmentManifest(data []byte) (CommitmentManifest, error) {
	var value CommitmentManifest
	if err := decodeStrict(data, &value); err != nil {
		return CommitmentManifest{}, err
	}
	return value, value.Validate()
}

// ScenarioProjection returns the documented pre-oracle-digest projection used
// to break the scenario/oracle digest cycle. It must be used only to bind the
// oracle to a scenario before its final oracle reference is populated.
func ScenarioProjection(manifest ScenarioManifest) ScenarioManifest {
	manifest.OracleRef.Digest = ""
	return manifest
}

// FinalizeScenarioOracle binds the oracle to the projected scenario, then the
// finalized scenario to the exact oracle digest. Neither digest can be swapped
// without invalidating the other side of the binding.
func FinalizeScenarioOracle(manifest ScenarioManifest, oracle OracleContract) (ScenarioManifest, OracleContract, error) {
	if manifest.Version != ScenarioVersion || manifest.OracleRef.Version != OracleVersion {
		return ScenarioManifest{}, OracleContract{}, fmt.Errorf("scenario and oracle versions are required")
	}
	scenarioDigest, err := Digest(ScenarioProjection(manifest))
	if err != nil {
		return ScenarioManifest{}, OracleContract{}, fmt.Errorf("digest scenario projection: %w", err)
	}
	oracle.ScenarioDigest = scenarioDigest
	if err := oracle.Validate(); err != nil {
		return ScenarioManifest{}, OracleContract{}, err
	}
	oracleDigest, err := Digest(oracle)
	if err != nil {
		return ScenarioManifest{}, OracleContract{}, fmt.Errorf("digest oracle: %w", err)
	}
	manifest.OracleRef.Digest = oracleDigest
	if err := manifest.Validate(); err != nil {
		return ScenarioManifest{}, OracleContract{}, err
	}
	return manifest, oracle, nil
}
