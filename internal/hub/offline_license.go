package hub

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/types"
)

const (
	offlineLicenseVersion               = "v1"
	offlineLicenseActivationsCollection = "offline_license_activations"
	offlineLicenseEnvPrivateKeyFile     = "LICENSE_PRIVATE_KEY_FILE"
	offlineLicenseEnvModelManifestFile  = "LICENSE_MODEL_MANIFEST"
)

type offlineActivationImportEnvelope struct {
	Content     string `json:"content"`
	SystemID    string `json:"systemId"`
	Customer    string `json:"customer"`
	Tenant      string `json:"tenant"`
	ProjectName string `json:"project_name"`
	SiteName    string `json:"site_name"`
	Remarks     string `json:"remarks"`
}

type offlineActivationRequest struct {
	Version            string            `json:"version"`
	RequestID          string            `json:"request_id"`
	GeneratedAt        string            `json:"generated_at"`
	Hostname           string            `json:"hostname"`
	Fingerprint        string            `json:"fingerprint"`
	Factors            map[string]string `json:"factors"`
	DevicePublicKeyB64 string            `json:"device_public_key_b64"`
}

type offlineLicenseIssueRequest struct {
	ActivationID string   `json:"activationId"`
	Customer     string   `json:"customer"`
	Tenant       string   `json:"tenant"`
	NotBefore    string   `json:"notBefore"`
	NotAfter     string   `json:"notAfter"`
	ModelNames   []string `json:"modelNames"`
}

type offlineLicensePayload struct {
	Version   string                      `json:"version"`
	RequestID string                      `json:"request_id"`
	Customer  string                      `json:"customer"`
	Tenant    string                      `json:"tenant,omitempty"`
	IssuedAt  string                      `json:"issued_at"`
	NotBefore string                      `json:"not_before,omitempty"`
	NotAfter  string                      `json:"not_after,omitempty"`
	Device    offlineLicenseDeviceBinding `json:"device"`
	Models    []offlineLicensedModelEntry `json:"models"`
}

type offlineLicenseDeviceBinding struct {
	Fingerprint  string            `json:"fingerprint"`
	Hostname     string            `json:"hostname,omitempty"`
	Factors      map[string]string `json:"factors"`
	PublicKeyPEM string            `json:"public_key_pem"`
}

type offlineLicensedModelEntry struct {
	Name         string `json:"name"`
	File         string `json:"file"`
	Sha256       string `json:"sha256"`
	WrappedKey   string `json:"wrapped_key"`
	Enabled      bool   `json:"enabled"`
	KeyAlgorithm string `json:"key_algorithm"`
	Cipher       string `json:"cipher"`
}

type offlineLicenseDocument struct {
	Payload   offlineLicensePayload `json:"payload"`
	Signature string                `json:"signature"`
}

type offlineLicenseManifest struct {
	Version string                      `json:"version"`
	Models  []offlineLicenseManifestRef `json:"models"`
}

type offlineLicenseManifestRef struct {
	Name    string `json:"name"`
	File    string `json:"file"`
	Sha256  string `json:"sha256"`
	KeyB64  string `json:"key_b64"`
	Enabled bool   `json:"enabled"`
}

type offlineLicenseExportResponse struct {
	LicenseID string `json:"licenseId"`
	FileName  string `json:"fileName"`
	Content   string `json:"content"`
}

type offlineLicenseSigningState struct {
	Ready      bool     `json:"ready"`
	Errors     []string `json:"errors"`
	ModelNames []string `json:"model_names"`
}

type offlineLicenseOverviewResponse struct {
	Ready       bool                             `json:"ready"`
	Signing     offlineLicenseSigningState       `json:"signing"`
	Activations []offlineLicenseActivationRecord `json:"activations"`
}

type offlineLicenseActivationPreviewResponse struct {
	RequestID          string                          `json:"request_id"`
	Hostname           string                          `json:"hostname"`
	Fingerprint        string                          `json:"fingerprint"`
	Factors            map[string]string               `json:"factors"`
	DevicePublicKey    string                          `json:"device_public_key_pem"`
	ExistingActivation *offlineLicenseActivationRecord `json:"existing_activation,omitempty"`
}

type offlineLicenseActivationRecord struct {
	ID                      string                      `json:"id"`
	RequestID               string                      `json:"request_id"`
	System                  string                      `json:"system,omitempty"`
	Customer                string                      `json:"customer"`
	Tenant                  string                      `json:"tenant"`
	ProjectName             string                      `json:"project_name"`
	SiteName                string                      `json:"site_name"`
	Remarks                 string                      `json:"remarks"`
	Fingerprint             string                      `json:"fingerprint"`
	Hostname                string                      `json:"hostname"`
	FactorsJSON             map[string]string           `json:"factors_json"`
	DevicePublicKey         string                      `json:"device_public_key_pem"`
	ActivationRaw           any                         `json:"activation_payload"`
	Status                  string                      `json:"status"`
	LastIssuedAt            string                      `json:"last_issued_at,omitempty"`
	CurrentLicenseID        string                      `json:"current_license_id,omitempty"`
	CurrentExportName       string                      `json:"current_export_name,omitempty"`
	CurrentNotBefore        string                      `json:"current_not_before,omitempty"`
	CurrentNotAfter         string                      `json:"current_not_after,omitempty"`
	CurrentModelsJSON       []offlineLicensedModelEntry `json:"current_models_json,omitempty"`
	CurrentLicensePayload   any                         `json:"current_license_payload,omitempty"`
	CurrentLicenseSignature string                      `json:"current_license_signature,omitempty"`
	Created                 string                      `json:"created"`
	Updated                 string                      `json:"updated"`
}

type preparedOfflineActivation struct {
	Activation           offlineActivationRequest
	SystemRecord         *core.Record
	ExistingRecord       *core.Record
	NormalizedFactors    map[string]string
	DevicePublicKeyPEM   string
	ActivationPayloadRaw types.JSONRaw
	FactorsRaw           types.JSONRaw
}

func (h *Hub) downloadOfflineLicenseCollector(e *core.RequestEvent) error {
	script := offlineLicenseCollectorScript()
	e.Response.Header().Set("Content-Type", "text/x-shellscript; charset=utf-8")
	e.Response.Header().Set("Content-Disposition", "attachment; filename=\"i3d-license-collector.sh\"")
	e.Response.WriteHeader(http.StatusOK)
	_, err := e.Response.Write([]byte(script))
	return err
}

func (h *Hub) previewOfflineLicenseActivation(e *core.RequestEvent) error {
	body, err := io.ReadAll(e.Request.Body)
	if err != nil {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "invalid body"})
	}

	envelope, err := parseOfflineActivationImportBody(body)
	if err != nil {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	prepared, err := h.prepareOfflineActivation(envelope)
	if err != nil {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	preview := offlineLicenseActivationPreviewResponse{
		RequestID:       prepared.Activation.RequestID,
		Hostname:        prepared.Activation.Hostname,
		Fingerprint:     prepared.Activation.Fingerprint,
		Factors:         prepared.NormalizedFactors,
		DevicePublicKey: prepared.DevicePublicKeyPEM,
	}
	if prepared.ExistingRecord != nil {
		existing, buildErr := h.buildOfflineLicenseActivationRecord(prepared.ExistingRecord)
		if buildErr != nil {
			return e.JSON(http.StatusInternalServerError, map[string]string{"error": buildErr.Error()})
		}
		preview.ExistingActivation = &existing
	}

	return e.JSON(http.StatusOK, preview)
}

func (h *Hub) importOfflineLicenseActivation(e *core.RequestEvent) error {
	body, err := io.ReadAll(e.Request.Body)
	if err != nil {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "invalid body"})
	}

	envelope, err := parseOfflineActivationImportBody(body)
	if err != nil {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	if strings.TrimSpace(envelope.Customer) == "" {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "customer is required"})
	}

	prepared, err := h.prepareOfflineActivation(envelope)
	if err != nil {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	collection, err := h.FindCollectionByNameOrId(offlineLicenseActivationsCollection)
	if err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	var record *core.Record
	action := "created"
	if prepared.ExistingRecord != nil {
		record = prepared.ExistingRecord
		action = "updated"
	} else {
		record = core.NewRecord(collection)
	}

	record.Set("request_id", prepared.Activation.RequestID)
	record.Set("customer", strings.TrimSpace(envelope.Customer))
	record.Set("tenant", strings.TrimSpace(envelope.Tenant))
	record.Set("project_name", strings.TrimSpace(envelope.ProjectName))
	record.Set("site_name", strings.TrimSpace(envelope.SiteName))
	record.Set("remarks", strings.TrimSpace(envelope.Remarks))
	record.Set("fingerprint", prepared.Activation.Fingerprint)
	record.Set("hostname", strings.TrimSpace(prepared.Activation.Hostname))
	record.Set("factors_json", prepared.FactorsRaw)
	record.Set("device_public_key_pem", prepared.DevicePublicKeyPEM)
	record.Set("activation_payload", prepared.ActivationPayloadRaw)
	if strings.TrimSpace(record.GetString("status")) == "" {
		record.Set("status", "imported")
	}
	if prepared.SystemRecord != nil {
		record.Set("system", prepared.SystemRecord.Id)
	}

	if err := h.Save(record); err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	activationRecord, err := h.buildOfflineLicenseActivationRecord(record)
	if err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return e.JSON(http.StatusOK, map[string]any{
		"id":          record.Id,
		"action":      action,
		"requestId":   prepared.Activation.RequestID,
		"fingerprint": prepared.Activation.Fingerprint,
		"hostname":    prepared.Activation.Hostname,
		"factors":     prepared.NormalizedFactors,
		"activation":  activationRecord,
	})
}

func (h *Hub) getOfflineLicenseOverview(e *core.RequestEvent) error {
	signingState := inspectOfflineLicenseSigningState()
	ready, err := h.areOfflineLicenseCollectionsReady()
	if err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	if !ready {
		return e.JSON(http.StatusOK, offlineLicenseOverviewResponse{
			Ready:       false,
			Signing:     signingState,
			Activations: []offlineLicenseActivationRecord{},
		})
	}

	activations, err := h.listOfflineLicenseActivations()
	if err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return e.JSON(http.StatusOK, offlineLicenseOverviewResponse{
		Ready:       true,
		Signing:     signingState,
		Activations: activations,
	})
}

func (h *Hub) issueOfflineLicense(e *core.RequestEvent) error {
	var payload offlineLicenseIssueRequest
	if err := json.NewDecoder(e.Request.Body).Decode(&payload); err != nil {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "invalid body"})
	}

	payload.ActivationID = strings.TrimSpace(payload.ActivationID)
	payload.Customer = strings.TrimSpace(payload.Customer)
	payload.Tenant = strings.TrimSpace(payload.Tenant)
	if payload.ActivationID == "" {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "activationId is required"})
	}

	notBefore, err := normalizeOfflineLicenseTime(payload.NotBefore)
	if err != nil {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	notAfter, err := normalizeOfflineLicenseTime(payload.NotAfter)
	if err != nil {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	if notBefore != "" && notAfter != "" && notBefore > notAfter {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "notBefore must be earlier than notAfter"})
	}

	activationRecord, err := h.FindRecordById(offlineLicenseActivationsCollection, payload.ActivationID)
	if err != nil {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "activation not found"})
	}
	if status := strings.TrimSpace(activationRecord.GetString("status")); status == "disabled" || status == "revoked" {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "activation is disabled"})
	}

	var activation offlineActivationRequest
	if err := activationRecord.UnmarshalJSONField("activation_payload", &activation); err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("decode activation payload failed: %v", err)})
	}

	customer := strings.TrimSpace(payload.Customer)
	if customer == "" {
		customer = strings.TrimSpace(activationRecord.GetString("customer"))
	}
	tenant := strings.TrimSpace(payload.Tenant)
	if tenant == "" {
		tenant = strings.TrimSpace(activationRecord.GetString("tenant"))
	}
	if customer == "" {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "customer is required"})
	}

	signingState := inspectOfflineLicenseSigningState()
	if !signingState.Ready {
		return e.JSON(http.StatusPreconditionFailed, map[string]string{"error": strings.Join(signingState.Errors, "; ")})
	}

	signingKey, err := loadOfflineLicenseSigningKey()
	if err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	manifest, err := loadOfflineLicenseManifest()
	if err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	models, err := selectOfflineLicenseModels(manifest, payload.ModelNames)
	if err != nil {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	devicePublicKeyPEM := activationRecord.GetString("device_public_key_pem")
	licenseModels := make([]offlineLicensedModelEntry, 0, len(models))
	for _, model := range models {
		wrappedKey, wrapErr := wrapOfflineLicenseModelKey(devicePublicKeyPEM, model.KeyB64, model.Name)
		if wrapErr != nil {
			return e.JSON(http.StatusInternalServerError, map[string]string{"error": wrapErr.Error()})
		}
		licenseModels = append(licenseModels, offlineLicensedModelEntry{
			Name:         model.Name,
			File:         model.File,
			Sha256:       model.Sha256,
			WrappedKey:   wrappedKey,
			Enabled:      model.Enabled,
			KeyAlgorithm: "rsa-oaep-sha256",
			Cipher:       "aes-256-gcm",
		})
	}

	licensePayload := offlineLicensePayload{
		Version:   offlineLicenseVersion,
		RequestID: activation.RequestID,
		Customer:  customer,
		Tenant:    tenant,
		IssuedAt:  time.Now().UTC().Format(time.RFC3339),
		NotBefore: notBefore,
		NotAfter:  notAfter,
		Device: offlineLicenseDeviceBinding{
			Fingerprint:  activation.Fingerprint,
			Hostname:     strings.TrimSpace(activation.Hostname),
			Factors:      normalizeOfflineLicenseFactors(activation.Factors),
			PublicKeyPEM: devicePublicKeyPEM,
		},
		Models: licenseModels,
	}
	signature, err := signOfflineLicensePayload(licensePayload, signingKey)
	if err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	licensePayloadRaw, err := toOfflineLicenseJSONRaw(licensePayload)
	if err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	modelsRaw, err := toOfflineLicenseJSONRaw(licenseModels)
	if err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	licenseID := fmt.Sprintf("%s-%d", activationRecord.Id, time.Now().UTC().UnixNano())

	activationRecord.Set("customer", customer)
	activationRecord.Set("tenant", tenant)
	activationRecord.Set("status", "active")
	activationRecord.Set("last_issued_at", licensePayload.IssuedAt)
	activationRecord.Set("current_license_id", licenseID)
	activationRecord.Set("current_export_name", offlineLicenseFileName(activation.RequestID))
	activationRecord.Set("current_not_before", notBefore)
	activationRecord.Set("current_not_after", notAfter)
	activationRecord.Set("current_models_json", modelsRaw)
	activationRecord.Set("current_license_payload", licensePayloadRaw)
	activationRecord.Set("current_license_signature", signature)
	if err := h.SaveNoValidate(activationRecord); err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return e.JSON(http.StatusOK, map[string]any{
		"licenseId":  licenseID,
		"requestId":  activation.RequestID,
		"fileName":   activationRecord.GetString("current_export_name"),
		"modelCount": len(licenseModels),
		"models":     licenseModels,
	})
}

func (h *Hub) exportOfflineLicense(e *core.RequestEvent) error {
	licenseID := strings.TrimSpace(e.Request.URL.Query().Get("licenseId"))
	if licenseID == "" {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "licenseId is required"})
	}

	records, err := h.FindRecordsByFilter(
		offlineLicenseActivationsCollection,
		"current_license_id={:license_id}",
		"-updated",
		1,
		0,
		dbx.Params{"license_id": licenseID},
	)
	if err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	if len(records) == 0 {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "license not found"})
	}
	record := records[0]

	var payload offlineLicensePayload
	if err := record.UnmarshalJSONField("current_license_payload", &payload); err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("decode license payload failed: %v", err)})
	}
	document := offlineLicenseDocument{
		Payload:   payload,
		Signature: record.GetString("current_license_signature"),
	}
	encoded, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	response := offlineLicenseExportResponse{
		LicenseID: licenseID,
		FileName:  record.GetString("current_export_name"),
		Content:   string(encoded),
	}
	if strings.TrimSpace(response.FileName) == "" {
		response.FileName = offlineLicenseFileName(payload.RequestID)
	}
	return e.JSON(http.StatusOK, response)
}

func (h *Hub) areOfflineLicenseCollectionsReady() (bool, error) {
	collections, err := h.App.FindAllCollections()
	if err != nil {
		return false, err
	}
	required := map[string]bool{
		offlineLicenseActivationsCollection: false,
	}
	for _, collection := range collections {
		if _, ok := required[collection.Name]; ok {
			required[collection.Name] = true
		}
	}
	for _, ready := range required {
		if !ready {
			return false, nil
		}
	}
	return true, nil
}

func (h *Hub) listOfflineLicenseActivations() ([]offlineLicenseActivationRecord, error) {
	records, err := h.FindRecordsByFilter(offlineLicenseActivationsCollection, "", "-updated", -1, 0, nil)
	if err != nil {
		return nil, err
	}

	items := make([]offlineLicenseActivationRecord, 0, len(records))
	for _, record := range records {
		item, buildErr := h.buildOfflineLicenseActivationRecord(record)
		if buildErr != nil {
			return nil, buildErr
		}
		items = append(items, item)
	}

	return items, nil
}

func (h *Hub) buildOfflineLicenseActivationRecord(record *core.Record) (offlineLicenseActivationRecord, error) {
	factors := map[string]string{}
	if err := record.UnmarshalJSONField("factors_json", &factors); err != nil {
		return offlineLicenseActivationRecord{}, fmt.Errorf("decode activation factors failed: %w", err)
	}
	var activationPayload any
	if err := record.UnmarshalJSONField("activation_payload", &activationPayload); err != nil {
		return offlineLicenseActivationRecord{}, fmt.Errorf("decode activation payload failed: %w", err)
	}
	currentLicenseID := strings.TrimSpace(record.GetString("current_license_id"))
	currentExportName := strings.TrimSpace(record.GetString("current_export_name"))
	currentLicenseSignature := strings.TrimSpace(record.GetString("current_license_signature"))
	currentModels := []offlineLicensedModelEntry{}
	var currentLicensePayload any
	if currentLicenseID != "" {
		if err := record.UnmarshalJSONField("current_models_json", &currentModels); err != nil {
			return offlineLicenseActivationRecord{}, fmt.Errorf("decode current license models failed: %w", err)
		}
		if err := record.UnmarshalJSONField("current_license_payload", &currentLicensePayload); err != nil {
			return offlineLicenseActivationRecord{}, fmt.Errorf("decode current license payload failed: %w", err)
		}
	}

	return offlineLicenseActivationRecord{
		ID:                      record.Id,
		RequestID:               record.GetString("request_id"),
		System:                  record.GetString("system"),
		Customer:                record.GetString("customer"),
		Tenant:                  record.GetString("tenant"),
		ProjectName:             record.GetString("project_name"),
		SiteName:                record.GetString("site_name"),
		Remarks:                 record.GetString("remarks"),
		Fingerprint:             record.GetString("fingerprint"),
		Hostname:                record.GetString("hostname"),
		FactorsJSON:             factors,
		DevicePublicKey:         record.GetString("device_public_key_pem"),
		ActivationRaw:           activationPayload,
		Status:                  record.GetString("status"),
		LastIssuedAt:            record.GetString("last_issued_at"),
		CurrentLicenseID:        currentLicenseID,
		CurrentExportName:       currentExportName,
		CurrentNotBefore:        record.GetString("current_not_before"),
		CurrentNotAfter:         record.GetString("current_not_after"),
		CurrentModelsJSON:       currentModels,
		CurrentLicensePayload:   currentLicensePayload,
		CurrentLicenseSignature: currentLicenseSignature,
		Created:                 record.GetString("created"),
		Updated:                 record.GetString("updated"),
	}, nil
}

func parseOfflineActivationImportBody(body []byte) (offlineActivationImportEnvelope, error) {
	trimmed := strings.TrimSpace(string(body))
	if trimmed == "" {
		return offlineActivationImportEnvelope{}, errors.New("activation content is required")
	}

	var envelope offlineActivationImportEnvelope
	if err := json.Unmarshal(body, &envelope); err == nil && strings.TrimSpace(envelope.Content) != "" {
		envelope.Content = strings.TrimSpace(envelope.Content)
		envelope.SystemID = strings.TrimSpace(envelope.SystemID)
		envelope.Customer = strings.TrimSpace(envelope.Customer)
		envelope.Tenant = strings.TrimSpace(envelope.Tenant)
		envelope.ProjectName = strings.TrimSpace(envelope.ProjectName)
		envelope.SiteName = strings.TrimSpace(envelope.SiteName)
		envelope.Remarks = strings.TrimSpace(envelope.Remarks)
		return envelope, nil
	}

	return offlineActivationImportEnvelope{Content: trimmed}, nil
}

func parseOfflineActivationRequest(raw string) (offlineActivationRequest, error) {
	var payload offlineActivationRequest
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return payload, fmt.Errorf("invalid activation request json: %w", err)
	}
	payload.RequestID = strings.TrimSpace(payload.RequestID)
	payload.Hostname = strings.TrimSpace(payload.Hostname)
	payload.Fingerprint = strings.TrimSpace(payload.Fingerprint)
	payload.DevicePublicKeyB64 = strings.TrimSpace(payload.DevicePublicKeyB64)
	if payload.RequestID == "" {
		return payload, errors.New("activation request_id is required")
	}
	if payload.DevicePublicKeyB64 == "" {
		return payload, errors.New("activation device_public_key_b64 is required")
	}
	return payload, nil
}

func decodeOfflineLicensePublicKey(encoded string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(encoded))
	if err != nil {
		return "", fmt.Errorf("decode device public key failed: %w", err)
	}
	normalized, err := normalizeOfflineLicensePEM(raw)
	if err != nil {
		return "", err
	}
	return normalized, nil
}

func normalizeOfflineLicensePEM(raw []byte) (string, error) {
	block, _ := pem.Decode(raw)
	if block == nil {
		return "", errors.New("invalid pem content")
	}
	publicKey, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return "", fmt.Errorf("parse public key failed: %w", err)
	}
	encoded, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		return "", fmt.Errorf("encode public key failed: %w", err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: encoded})), nil
}

func normalizeOfflineLicenseFactors(factors map[string]string) map[string]string {
	normalized := map[string]string{
		"machine_id":   "",
		"product_uuid": "",
		"board_serial": "",
	}
	for key := range normalized {
		normalized[key] = strings.TrimSpace(factors[key])
	}
	return normalized
}

func (h *Hub) prepareOfflineActivation(envelope offlineActivationImportEnvelope) (preparedOfflineActivation, error) {
	activation, err := parseOfflineActivationRequest(envelope.Content)
	if err != nil {
		return preparedOfflineActivation{}, err
	}
	if activation.RequestID == "" {
		return preparedOfflineActivation{}, errors.New("activation request_id is required")
	}

	devicePublicKeyPEM, err := decodeOfflineLicensePublicKey(activation.DevicePublicKeyB64)
	if err != nil {
		return preparedOfflineActivation{}, err
	}

	normalizedFactors := normalizeOfflineLicenseFactors(activation.Factors)
	expectedFingerprint := offlineLicenseFingerprint(normalizedFactors)
	if activation.Fingerprint == "" {
		activation.Fingerprint = expectedFingerprint
	}
	if activation.Fingerprint != expectedFingerprint {
		return preparedOfflineActivation{}, errors.New("activation fingerprint mismatch")
	}

	activationPayloadRaw, err := toOfflineLicenseJSONRaw(activation)
	if err != nil {
		return preparedOfflineActivation{}, err
	}
	factorsRaw, err := toOfflineLicenseJSONRaw(normalizedFactors)
	if err != nil {
		return preparedOfflineActivation{}, err
	}

	var systemRecord *core.Record
	if envelope.SystemID != "" {
		systemRecord, err = h.FindRecordById("systems", envelope.SystemID)
		if err != nil {
			return preparedOfflineActivation{}, errors.New("system not found")
		}
	}

	records, err := h.FindRecordsByFilter(
		offlineLicenseActivationsCollection,
		"request_id={:request_id}",
		"-updated",
		1,
		0,
		dbx.Params{"request_id": activation.RequestID},
	)
	if err != nil {
		return preparedOfflineActivation{}, err
	}

	var existingRecord *core.Record
	if len(records) > 0 {
		existingRecord = records[0]
	}

	return preparedOfflineActivation{
		Activation:           activation,
		SystemRecord:         systemRecord,
		ExistingRecord:       existingRecord,
		NormalizedFactors:    normalizedFactors,
		DevicePublicKeyPEM:   devicePublicKeyPEM,
		ActivationPayloadRaw: activationPayloadRaw,
		FactorsRaw:           factorsRaw,
	}, nil
}

func offlineLicenseFingerprint(factors map[string]string) string {
	keys := make([]string, 0, len(factors))
	for key := range factors {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	builder := strings.Builder{}
	for _, key := range keys {
		builder.WriteString(key)
		builder.WriteString("=")
		builder.WriteString(strings.TrimSpace(factors[key]))
		builder.WriteString("\n")
	}
	return fmt.Sprintf("%x", sha256.Sum256([]byte(builder.String())))
}

func loadOfflineLicenseSigningKey() (ed25519.PrivateKey, error) {
	keyPath, exists := GetEnv(offlineLicenseEnvPrivateKeyFile)
	if !exists || strings.TrimSpace(keyPath) == "" {
		return nil, fmt.Errorf("missing env AETHER_HUB_%s", offlineLicenseEnvPrivateKeyFile)
	}

	raw, err := os.ReadFile(filepath.Clean(keyPath))
	if err != nil {
		return nil, fmt.Errorf("read signing private key failed: %w", err)
	}
	block, _ := pem.Decode(raw)
	if block == nil {
		return nil, errors.New("invalid signing private key pem")
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse signing private key failed: %w", err)
	}
	key, ok := parsed.(ed25519.PrivateKey)
	if !ok {
		return nil, errors.New("signing private key must be Ed25519 PKCS8")
	}
	return key, nil
}

func inspectOfflineLicenseSigningState() offlineLicenseSigningState {
	state := offlineLicenseSigningState{
		Ready:      false,
		Errors:     []string{},
		ModelNames: []string{},
	}

	if _, err := loadOfflineLicenseSigningKey(); err != nil {
		state.Errors = append(state.Errors, err.Error())
	}

	manifest, err := loadOfflineLicenseManifest()
	if err != nil {
		state.Errors = append(state.Errors, err.Error())
	} else {
		for _, model := range manifest.Models {
			state.ModelNames = append(state.ModelNames, model.Name)
		}
	}

	state.Ready = len(state.Errors) == 0
	return state
}

func loadOfflineLicenseManifest() (offlineLicenseManifest, error) {
	var manifest offlineLicenseManifest
	path, exists := GetEnv(offlineLicenseEnvModelManifestFile)
	if !exists || strings.TrimSpace(path) == "" {
		return manifest, fmt.Errorf("missing env AETHER_HUB_%s", offlineLicenseEnvModelManifestFile)
	}
	raw, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return manifest, fmt.Errorf("read model manifest failed: %w", err)
	}
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return manifest, fmt.Errorf("parse model manifest failed: %w", err)
	}
	if len(manifest.Models) == 0 {
		return manifest, errors.New("model manifest is empty")
	}
	for index := range manifest.Models {
		manifest.Models[index].Name = strings.TrimSpace(manifest.Models[index].Name)
		manifest.Models[index].File = strings.TrimSpace(manifest.Models[index].File)
		manifest.Models[index].Sha256 = strings.TrimSpace(manifest.Models[index].Sha256)
		manifest.Models[index].KeyB64 = strings.TrimSpace(manifest.Models[index].KeyB64)
		if !manifest.Models[index].Enabled {
			manifest.Models[index].Enabled = true
		}
		if manifest.Models[index].Name == "" || manifest.Models[index].File == "" || manifest.Models[index].Sha256 == "" || manifest.Models[index].KeyB64 == "" {
			return manifest, fmt.Errorf("model manifest entry %d is incomplete", index)
		}
	}
	return manifest, nil
}

func selectOfflineLicenseModels(manifest offlineLicenseManifest, requestedNames []string) ([]offlineLicenseManifestRef, error) {
	if len(requestedNames) == 0 {
		return manifest.Models, nil
	}

	selected := make([]offlineLicenseManifestRef, 0, len(requestedNames))
	index := make(map[string]offlineLicenseManifestRef, len(manifest.Models))
	for _, model := range manifest.Models {
		index[model.Name] = model
	}

	for _, name := range requestedNames {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		model, ok := index[name]
		if !ok {
			return nil, fmt.Errorf("model not found in manifest: %s", name)
		}
		selected = append(selected, model)
	}
	if len(selected) == 0 {
		return nil, errors.New("no models selected")
	}
	return selected, nil
}

func wrapOfflineLicenseModelKey(devicePublicKeyPEM string, keyB64 string, label string) (string, error) {
	block, _ := pem.Decode([]byte(devicePublicKeyPEM))
	if block == nil {
		return "", errors.New("invalid device public key pem")
	}
	parsed, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return "", fmt.Errorf("parse device public key failed: %w", err)
	}
	publicKey, ok := parsed.(*rsa.PublicKey)
	if !ok {
		return "", errors.New("device public key must be RSA")
	}
	keyBytes, err := base64.StdEncoding.DecodeString(keyB64)
	if err != nil {
		return "", fmt.Errorf("decode model key failed for %s: %w", label, err)
	}
	wrapped, err := rsa.EncryptOAEP(sha256.New(), rand.Reader, publicKey, keyBytes, []byte(label))
	if err != nil {
		return "", fmt.Errorf("wrap model key failed for %s: %w", label, err)
	}
	return base64.StdEncoding.EncodeToString(wrapped), nil
}

func signOfflineLicensePayload(payload offlineLicensePayload, privateKey ed25519.PrivateKey) (string, error) {
	raw, err := marshalCanonicalJSON(payload)
	if err != nil {
		return "", err
	}
	signature := ed25519.Sign(privateKey, raw)
	return base64.StdEncoding.EncodeToString(signature), nil
}

func marshalCanonicalJSON(value any) ([]byte, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}

	var normalized any
	if err := json.Unmarshal(raw, &normalized); err != nil {
		return nil, err
	}

	var buffer bytes.Buffer
	if err := writeCanonicalJSON(&buffer, normalized); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func writeCanonicalJSON(buffer *bytes.Buffer, value any) error {
	switch current := value.(type) {
	case nil:
		buffer.WriteString("null")
	case bool:
		if current {
			buffer.WriteString("true")
		} else {
			buffer.WriteString("false")
		}
	case string:
		encoded, err := json.Marshal(current)
		if err != nil {
			return err
		}
		buffer.Write(encoded)
	case float64:
		encoded, err := json.Marshal(current)
		if err != nil {
			return err
		}
		buffer.Write(encoded)
	case []any:
		buffer.WriteByte('[')
		for index, item := range current {
			if index > 0 {
				buffer.WriteByte(',')
			}
			if err := writeCanonicalJSON(buffer, item); err != nil {
				return err
			}
		}
		buffer.WriteByte(']')
	case map[string]any:
		keys := make([]string, 0, len(current))
		for key := range current {
			keys = append(keys, key)
		}
		sort.Strings(keys)

		buffer.WriteByte('{')
		for index, key := range keys {
			if index > 0 {
				buffer.WriteByte(',')
			}
			encodedKey, err := json.Marshal(key)
			if err != nil {
				return err
			}
			buffer.Write(encodedKey)
			buffer.WriteByte(':')
			if err := writeCanonicalJSON(buffer, current[key]); err != nil {
				return err
			}
		}
		buffer.WriteByte('}')
	default:
		encoded, err := json.Marshal(current)
		if err != nil {
			return err
		}
		buffer.Write(encoded)
	}
	return nil
}

func normalizeOfflineLicenseTime(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}

	layouts := []string{
		time.RFC3339,
		"2006-01-02",
	}
	for _, layout := range layouts {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed.UTC().Format(time.RFC3339), nil
		}
	}
	return "", fmt.Errorf("invalid time format: %s", value)
}

func offlineLicenseFileName(requestID string) string {
	normalized := strings.ReplaceAll(strings.TrimSpace(requestID), "/", "-")
	if normalized == "" {
		normalized = "license"
	}
	return fmt.Sprintf("%s.dat", normalized)
}

func toOfflineLicenseJSONRaw(value any) (types.JSONRaw, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return types.JSONRaw(encoded), nil
}

func offlineLicenseCollectorScript() string {
	return `#!/usr/bin/env bash
set -euo pipefail

TARGET_DIR="${1:-/var/lib/i3d/license}"
mkdir -p "${TARGET_DIR}"
chmod 700 "${TARGET_DIR}"

json_escape() {
  printf '%s' "$1" | sed 's/\\/\\\\/g; s/"/\\"/g'
}

sha256_stdin() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum | awk '{print $1}'
    return
  fi
  if command -v shasum >/dev/null 2>&1; then
    shasum -a 256 | awk '{print $1}'
    return
  fi
  printf 'missing sha256 command\n' >&2
  exit 1
}

base64_file() {
  base64 "$1" | tr -d '\n'
}

read_first_existing() {
  local value=""
  for path in "$@"; do
    if [ -f "${path}" ]; then
      value="$(tr -d '\r\n' < "${path}")"
      if [ -n "${value}" ]; then
        printf '%s' "${value}"
        return 0
      fi
    fi
  done
  printf ''
}

if [ ! -f "${TARGET_DIR}/device.key" ]; then
  openssl genpkey -algorithm RSA -pkeyopt rsa_keygen_bits:3072 -out "${TARGET_DIR}/device.key"
  chmod 600 "${TARGET_DIR}/device.key"
fi

openssl pkey -in "${TARGET_DIR}/device.key" -pubout -out "${TARGET_DIR}/device.pub" >/dev/null 2>&1
chmod 644 "${TARGET_DIR}/device.pub"

MACHINE_ID="$(read_first_existing /etc/machine-id /var/lib/dbus/machine-id)"
PRODUCT_UUID="$(read_first_existing /sys/class/dmi/id/product_uuid)"
BOARD_SERIAL="$(read_first_existing /sys/class/dmi/id/board_serial)"
HOSTNAME_VALUE="$(hostname 2>/dev/null || printf '')"
REQUEST_ID="activation-$(date -u +%Y%m%d%H%M%S)-$(openssl rand -hex 6)"
GENERATED_AT="$(date -u +%Y-%m-%dT%H:%M:%SZ)"

FINGERPRINT="$(printf 'board_serial=%s\nmachine_id=%s\nproduct_uuid=%s\n' "${BOARD_SERIAL}" "${MACHINE_ID}" "${PRODUCT_UUID}" | sha256_stdin)"
DEVICE_PUBLIC_KEY_B64="$(base64_file "${TARGET_DIR}/device.pub")"

cat > "${TARGET_DIR}/activation.req" <<EOF_JSON
{
  "version": "v1",
  "request_id": "$(json_escape "${REQUEST_ID}")",
  "generated_at": "$(json_escape "${GENERATED_AT}")",
  "hostname": "$(json_escape "${HOSTNAME_VALUE}")",
  "fingerprint": "$(json_escape "${FINGERPRINT}")",
  "factors": {
    "machine_id": "$(json_escape "${MACHINE_ID}")",
    "product_uuid": "$(json_escape "${PRODUCT_UUID}")",
    "board_serial": "$(json_escape "${BOARD_SERIAL}")"
  },
  "device_public_key_b64": "$(json_escape "${DEVICE_PUBLIC_KEY_B64}")"
}
EOF_JSON

printf 'activation request generated: %s\n' "${TARGET_DIR}/activation.req"
printf 'private key saved: %s\n' "${TARGET_DIR}/device.key"
printf 'public key saved: %s\n' "${TARGET_DIR}/device.pub"
`
}
