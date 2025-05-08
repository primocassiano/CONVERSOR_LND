package main

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"aezeed_address_generator_gui/internal/crypto" // Import the local crypto package

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout" // <<< Added for Spacer
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/btcsuite/btcd/btcjson"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/btcutil/hdkeychain"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/rpcclient"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
)

// Constants
const (
	BIP44Purpose uint32 = 44
	BIP49Purpose uint32 = 49
	BIP84Purpose uint32 = 84
	BIP86Purpose uint32 = 86
	CoinTypeBitcoin uint32 = 0
	DefaultAccount uint32 = 0
	ExternalChain uint32 = 0
	InternalChain uint32 = 1
	AddressBatchSize = 20

	SourceOffline = "Offline"
	SourceBlockstream = "Blockstream.info (Público)"
	SourceLocalNode = "Nó Local (RPC)"

	apiCallDelay = 100 * time.Millisecond
	addressSearchLimit uint32 = 20000
)

// Global Variables
var (
	myApp fyne.App
	currentChangeType uint32 = ExternalChain
	currentBatchStart uint32 = 0
	currentMasterKey *hdkeychain.ExtendedKey
	netParams = &chaincfg.MainNetParams
	mainWindow fyne.Window

	selectedBlockchainSource = SourceOffline
	localNodeURL = "127.0.0.1:8332"
	localNodeUser = ""
	localNodePass = ""
	localNodeClient *rpcclient.Client
	clientMutex sync.Mutex
	lastRpcHost string
	lastRpcUser string
	lastRpcPass string

	// UI Elements
	passphraseEntry *widget.Entry
	mnemonicEntry *widget.Entry
	blockchainSourceRadio *widget.RadioGroup
	localNodeURLEntry *widget.Entry
	localNodeUserEntry *widget.Entry
	localNodePassEntry *widget.Entry
	localNodeConfigCard *widget.Card // <<< Changed to Card
	statusBinding binding.String
	addressLookupEntry *widget.Entry
	addressLookupButton *widget.Button // <<< Added
	 xpubContainer *fyne.Container
	 outputContainer *fyne.Container
	 batchLabel *widget.Label
	 loadMoreButton *widget.Button
	 verifyLegacyButton *widget.Button
	 verifyNestedButton *widget.Button
	 verifyNativeButton *widget.Button
	 verifyTaprootButton *widget.Button
	 verificationButtons *fyne.Container
	 generateButton *widget.Button
	 decodeButton *widget.Button
	 accountToggleButton *widget.Button
	progressBar *widget.ProgressBarInfinite
)

// --- Blockchain Interaction Logic (getRPCClient, checkAddressBlockstream) ---
// ... (No changes needed in these functions for visual improvements)

// getRPCClient establishes or returns an existing RPC client connection.
func getRPCClient() (*rpcclient.Client, error) {
	clientMutex.Lock()
	defer clientMutex.Unlock()

	configChanged := localNodeURL != lastRpcHost || localNodeUser != lastRpcUser || localNodePass != lastRpcPass

	 if localNodeClient != nil && !configChanged {
		 err := localNodeClient.Ping()
		 if err == nil {
			 return localNodeClient, nil
		 }
		 log.Printf("RPC Ping failed: %v. Reconnecting...", err)
		 localNodeClient.Shutdown()
		 localNodeClient = nil
	 }

	 if configChanged && localNodeClient != nil {
		 log.Println("RPC config changed. Reconnecting...")
		 localNodeClient.Shutdown()
		 localNodeClient = nil
	 }

	 log.Println("Creating new RPC client...")
	 connCfg := &rpcclient.ConnConfig{
		 Host:         localNodeURL,
		 User:         localNodeUser,
		 Pass:         localNodePass,
		 HTTPPostMode: true,
		 DisableTLS:   true,
	 }
	 if strings.HasPrefix(strings.ToLower(localNodeURL), "https://") {
		 connCfg.DisableTLS = false
		 connCfg.Host = strings.TrimPrefix(localNodeURL, "https://")
	 } else if strings.HasPrefix(strings.ToLower(localNodeURL), "http://") {
		 connCfg.Host = strings.TrimPrefix(localNodeURL, "http://")
	 }

	 var err error
	 localNodeClient, err = rpcclient.New(connCfg, nil)
	 if err != nil {
		 localNodeClient = nil
		 errMsg := fmt.Sprintf("Erro ao conectar ao nó RPC em ", connCfg.Host, err)
		 if strings.Contains(err.Error(), "connection refused") {
			 errMsg = fmt.Sprintf("Erro: Conexão recusada pelo nó RPC em ", connCfg.Host, ". Verifique se o nó está rodando e a URL/porta está correta.")
		 } else if strings.Contains(err.Error(), "401 Unauthorized") {
			 errMsg = fmt.Sprintf("Erro: Falha na autenticação RPC (usuário/senha incorretos) para ", connCfg.Host, ". Verifique suas credenciais.")
		 } else if strings.Contains(err.Error(), "no such host") {
			 errMsg = fmt.Sprintf("Erro: Host RPC ", connCfg.Host, " não encontrado. Verifique a URL.")
		 }
		 return nil, fmt.Errorf(errMsg)
	 }

	 lastRpcHost = localNodeURL
	 lastRpcUser = localNodeUser
	 lastRpcPass = localNodePass

	 return localNodeClient, nil
}

// checkAddressBlockstream retrieves transaction count for an address from Blockstream.info API.
func checkAddressBlockstream(address string) (string, error) {
	apiURL := fmt.Sprintf("https://blockstream.info/api/address/%s", address)
	resp, err := http.Get(apiURL)
	 if err != nil {
		 errMsg := fmt.Sprintf("Erro ao conectar à API Blockstream para o endereço %s: %v", address, err)
		 if strings.Contains(err.Error(), "no such host") {
			 errMsg = "Erro: Não foi possível encontrar o host da API Blockstream (blockstream.info). Verifique sua conexão com a internet."
		 } else if strings.Contains(err.Error(), "timeout") {
			 errMsg = "Erro: Tempo limite excedido ao conectar à API Blockstream. Verifique sua conexão ou tente novamente mais tarde."
		 }
		 return "", fmt.Errorf(errMsg)
	 }
	 defer resp.Body.Close()

	 if resp.StatusCode != http.StatusOK {
		 bodyBytes, _ := io.ReadAll(resp.Body)
		 if resp.StatusCode == http.StatusNotFound {
			 return "Tx Count: 0", nil
		 }
		 errMsg := fmt.Sprintf("Erro da API Blockstream (%d) para o endereço %s: %s", resp.StatusCode, address, string(bodyBytes))
		 if resp.StatusCode == 429 {
			 errMsg = fmt.Sprintf("Erro: Muitas requisições para a API Blockstream (Rate Limit). Tente novamente mais tarde. (%d)", resp.StatusCode)
		 }
		 return "", fmt.Errorf(errMsg)
	 }

	 var addrInfo map[string]interface{}
	 if err := json.NewDecoder(resp.Body).Decode(&addrInfo); err != nil {
		 return "", fmt.Errorf("erro ao decodificar resposta da Blockstream: %w", err)
	 }

	 chainStats, okCS := addrInfo["chain_stats"].(map[string]interface{})
	 fundedTxoCount, okF := chainStats["funded_txo_count"].(float64)
	 spentTxoCount, okS := chainStats["spent_txo_count"].(float64)

	 if !okCS || !okF || !okS {
		 return "(Info de Tx não disponível)", nil
	 }

	 txCount := fundedTxoCount + spentTxoCount
	 return fmt.Sprintf("Tx Count: %.0f", txCount), nil
}

// Helper to marshal map to JSON for RawRequest
func marshalJSON(v interface{}) string {
	 bytes, _ := json.Marshal(v)
	 return string(bytes)
}

// --- Crypto Logic (deriveChildKey, deriveAccountXpub, generate*Address) ---
// ... (No changes needed in these functions for visual improvements)

// deriveChildKey derives a child key from an extended key based on the specified path components.
func deriveChildKey(masterKey *hdkeychain.ExtendedKey, purpose, coinType, account, chain, index uint32) (*hdkeychain.ExtendedKey, error) {
	purposeKey, err := masterKey.Derive(purpose + hdkeychain.HardenedKeyStart)
	 if err != nil {
		 return nil, fmt.Errorf("failed to derive purpose key: %w", err)
	 }
	 coinTypeKey, err := purposeKey.Derive(coinType + hdkeychain.HardenedKeyStart)
	 if err != nil {
		 return nil, fmt.Errorf("failed to derive coin type key: %w", err)
	 }
	 accountKey, err := coinTypeKey.Derive(account + hdkeychain.HardenedKeyStart)
	 if err != nil {
		 return nil, fmt.Errorf("failed to derive account key: %w", err)
	 }
	 chainKey, err := accountKey.Derive(chain)
	 if err != nil {
		 return nil, fmt.Errorf("failed to derive chain key: %w", err)
	 }
	 indexKey, err := chainKey.Derive(index)
	 if err != nil {
		 return nil, fmt.Errorf("failed to derive index key %d: %w", index, err)
	 }
	 return indexKey, nil
}

// deriveAccountXpub derives the account-level extended public key (XPUB) for a given purpose.
func deriveAccountXpub(masterKey *hdkeychain.ExtendedKey, purpose, coinType, account uint32, netParams *chaincfg.Params) (string, error) {
	purposeKey, err := masterKey.Derive(purpose + hdkeychain.HardenedKeyStart)
	 if err != nil {
		 return "", fmt.Errorf("failed to derive purpose key for xpub: %w", err)
	 }
	 coinTypeKey, err := purposeKey.Derive(coinType + hdkeychain.HardenedKeyStart)
	 if err != nil {
		 return "", fmt.Errorf("failed to derive coin type key for xpub: %w", err)
	 }
	 accountKey, err := coinTypeKey.Derive(account + hdkeychain.HardenedKeyStart)
	 if err != nil {
		 return "", fmt.Errorf("failed to derive account key for xpub: %w", err)
	 }
	 xpubKey, err := accountKey.Neuter()
	 if err != nil {
		 return "", fmt.Errorf("failed to neuter account key for xpub: %w", err)
	 }
	 xpubKey.SetNet(netParams)
	 return xpubKey.String(), nil
}

// generateLegacyAddress generates a P2PKH address from a derived key.
func generateLegacyAddress(key *hdkeychain.ExtendedKey, netParams *chaincfg.Params) (btcutil.Address, error) {
	pubKey, err := key.ECPubKey()
	 if err != nil {
		 return nil, fmt.Errorf("failed to get public key: %w", err)
	 }
	 return btcutil.NewAddressPubKeyHash(btcutil.Hash160(pubKey.SerializeCompressed()), netParams)
}

// generateNestedSegWitAddress generates a P2SH-P2WPKH address from a derived key.
func generateNestedSegWitAddress(key *hdkeychain.ExtendedKey, netParams *chaincfg.Params) (btcutil.Address, error) {
	pubKey, err := key.ECPubKey()
	 if err != nil {
		 return nil, fmt.Errorf("failed to get public key: %w", err)
	 }
	 pubKeyBytes := pubKey.SerializeCompressed()
	 pubKeyHash := btcutil.Hash160(pubKeyBytes)
	 builder := txscript.NewScriptBuilder()
	 builder.AddOp(txscript.OP_0)
	 builder.AddData(pubKeyHash)
	 witnessScript, err := builder.Script()
	 if err != nil {
		 return nil, fmt.Errorf("failed to build witness script: %w", err)
	 }
	 scriptHash := btcutil.Hash160(witnessScript)
	 return btcutil.NewAddressScriptHashFromHash(scriptHash, netParams)
}

// generateNativeSegWitAddress generates a P2WPKH address from a derived key.
func generateNativeSegWitAddress(key *hdkeychain.ExtendedKey, netParams *chaincfg.Params) (btcutil.Address, error) {
	pubKey, err := key.ECPubKey()
	 if err != nil {
		 return nil, fmt.Errorf("failed to get public key: %w", err)
	 }
	 pubKeyBytes := pubKey.SerializeCompressed()
	 pubKeyHash := btcutil.Hash160(pubKeyBytes)
	 return btcutil.NewAddressWitnessPubKeyHash(pubKeyHash, netParams)
}

// generateTaprootAddress generates a P2TR address from a derived key.
func generateTaprootAddress(key *hdkeychain.ExtendedKey, netParams *chaincfg.Params) (btcutil.Address, error) {
	pubKey, err := key.ECPubKey()
	 if err != nil {
		 return nil, fmt.Errorf("failed to get public key: %w", err)
	 }
	 taprootPubKey := txscript.ComputeTaprootKeyNoScript(pubKey)
	 return btcutil.NewAddressTaproot(schnorr.SerializePubKey(taprootPubKey), netParams)
}

// --- UI Logic ---

func main() {
	myApp = app.New()
	myWindow := myApp.NewWindow("Gerador de Endereços Aezeed v3.0") // <<< Version Bump
	mainWindow = myWindow

	// --- Input Area ---
	passphraseEntry = widget.NewPasswordEntry()
	passphraseEntry.SetPlaceHolder("Frase-senha (opcional, padrão 'aezeed')")

	mnemonicEntry = widget.NewMultiLineEntry()
	mnemonicEntry.SetPlaceHolder("Cole o mnemônico de 24 palavras aqui...")
	mnemonicEntry.Wrapping = fyne.TextWrapWord
	mnemonicEntry.SetMinRowsVisible(3)

	generateButton = widget.NewButtonWithIcon("Gerar Nova Seed", theme.ContentAddIcon(), func() {
		clearStatus()
		generateNewSeedAndAddresses()
	})

	decodeButton = widget.NewButtonWithIcon("Decodificar Mnemônico", theme.ConfirmIcon(), func() {
		clearStatus()
		decodeMnemonicAndAddresses()
	})

	accountToggleButton = widget.NewButton("Mostrar Endereços Internos (Change 1)", func() {
		 currentChangeType = 1 - currentChangeType
		 if currentChangeType == ExternalChain {
			 accountToggleButton.SetText("Mostrar Endereços Internos (Change 1)")
			 showStatus("Exibindo endereços Externos (change 0)", false)
		 } else {
			 accountToggleButton.SetText("Mostrar Endereços Externos (Change 0)")
			 showStatus("Exibindo endereços Internos (change 1)", false)
		 }
		 currentBatchStart = 0
		 updateAddressGrid()
	 })
	accountToggleButton.SetText("Mostrar Endereços Internos (Change 1)")

	// --- Blockchain Source Config ---
	localNodeURLEntry = widget.NewEntry()
	localNodeURLEntry.SetText(localNodeURL)
	localNodeURLEntry.OnChanged = func(s string) { localNodeURL = s }
	localNodeUserEntry = widget.NewEntry()
	localNodeUserEntry.SetPlaceHolder("Usuário RPC (opcional)")
	localNodeUserEntry.OnChanged = func(s string) { localNodeUser = s }
	localNodePassEntry = widget.NewPasswordEntry()
	localNodePassEntry.SetPlaceHolder("Senha RPC (opcional)")
	localNodePassEntry.OnChanged = func(s string) { localNodePass = s }

	// <<< Wrap local node config in a Card
	localNodeConfigCard = widget.NewCard("Configuração Nó Local (RPC)", "", container.NewVBox(
		widget.NewForm(
			widget.NewFormItem("URL:", localNodeURLEntry),
			widget.NewFormItem("Usuário:", localNodeUserEntry),
			widget.NewFormItem("Senha:", localNodePassEntry),
		),
	))
	localNodeConfigCard.Hide() // Hide initially

	blockchainSourceRadio = widget.NewRadioGroup([]string{SourceOffline, SourceBlockstream, SourceLocalNode}, func(selected string) {
		log.Printf("Fonte Blockchain selecionada: %s", selected)
		selectedBlockchainSource = selected
		 if selected == SourceLocalNode {
			localNodeConfigCard.Show()
		 } else {
			localNodeConfigCard.Hide()
		 }
		 if verificationButtons != nil {
			 if selected == SourceOffline {
				 verificationButtons.Hide()
			 } else {
				 verificationButtons.Show()
			 }
		 } else {
			 log.Println("WARN: verificationButtons is nil in RadioGroup callback")
		 }
	})
	blockchainSourceRadio.SetSelected(SourceOffline)

	blockchainConfigArea := container.NewVBox(
		widget.NewLabelWithStyle("Fonte de Dados Blockchain:", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		blockchainSourceRadio,
		localNodeConfigCard, // <<< Use Card here
	)

	// --- XPUB Display ---
	 xpubContainer = container.NewVBox(
		 widget.NewLabelWithStyle("Chaves Públicas Estendidas (XPUBs) da Conta 0:", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		 widget.NewLabel("Gere ou decodifique uma seed para ver as XPUBs."),
	 )

	// --- Status Label ---
	 statusBinding = binding.NewString()
	 statusLabel := widget.NewLabelWithData(statusBinding)
	 statusLabel.Wrapping = fyne.TextWrapWord

	// --- Progress Bar ---
	progressBar = widget.NewProgressBarInfinite()
	progressBar.Hide()

	// --- Address Lookup Area ---
	addressLookupEntry = widget.NewEntry()
	addressLookupEntry.SetPlaceHolder("Cole o endereço Bitcoin para buscar...")
	addressLookupButton = widget.NewButtonWithIcon("Buscar Endereço", theme.SearchIcon(), func() {
		 handleAddressLookup()
	})

	// --- Left Panel (Input/Config/XPUB/Status) ---
	// <<< Added Spacers and grouped sections
	leftPanel := container.NewVBox(
		widget.NewCard("Opção 1: Gerar Nova Seed", "", container.NewPadded( // <<< Add padding
			container.NewVBox(
				widget.NewForm(widget.NewFormItem("Passphrase:", passphraseEntry)),
				generateButton,
			),
		)),
			layout.NewSpacer(), // <<< Spacer
			widget.NewCard("Opção 2: Usar Mnemônico Existente", "", container.NewPadded( // <<< Add padding
				container.NewVBox(
					widget.NewLabel("Mnemônico (24 palavras):"),
					mnemonicEntry,
					decodeButton,
					accountToggleButton,
				),
			)),
			layout.NewSpacer(), // <<< Spacer
			blockchainConfigArea,
			layout.NewSpacer(), // <<< Spacer
			widget.NewCard("Buscar Endereço Individual", "", container.NewPadded( // <<< Add padding
				container.NewVBox(
					addressLookupEntry,
					addressLookupButton,
				),
			)),
		layout.NewSpacer(), // <<< Spacer
		 xpubContainer,
		layout.NewSpacer(), // <<< Spacer
		widget.NewLabelWithStyle("Status:", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		 statusLabel,
		 progressBar,
	)

	// --- Right Panel (Address Output & Verification) ---
	batchLabel = widget.NewLabel("Endereços: ")
	outputContainer = container.NewVBox(widget.NewLabel("Gere ou decodifique uma seed para ver os endereços."))

	loadMoreButton = widget.NewButtonWithIcon("Carregar Próximos 20", theme.NavigateNextIcon(), func() {
		clearStatus()
		loadNextBatch()
	})
	loadMoreButton.Disable()

	// --- Verification Buttons ---
	// <<< Added Icons to verification buttons
	 verifyLegacyButton = widget.NewButtonWithIcon("Verificar Legado", theme.InfoIcon(), func() { checkDerivationInfo(BIP44Purpose, "Legado (BIP44)") })
	 verifyNestedButton = widget.NewButtonWithIcon("Verificar Nested", theme.InfoIcon(), func() { checkDerivationInfo(BIP49Purpose, "Nested SegWit (BIP49)") })
	 verifyNativeButton = widget.NewButtonWithIcon("Verificar Nativo", theme.InfoIcon(), func() { checkDerivationInfo(BIP84Purpose, "SegWit Nativo (BIP84)") })
	 verifyTaprootButton = widget.NewButtonWithIcon("Verificar Taproot", theme.InfoIcon(), func() { checkDerivationInfo(BIP86Purpose, "Taproot (BIP86)") })

	 verificationButtons = container.NewVBox(
		 widget.NewLabelWithStyle("Verificar Uso dos Endereços Atuais:", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		 container.NewGridWithColumns(2,
			 verifyLegacyButton,
			 verifyNestedButton,
			 verifyNativeButton,
			 verifyTaprootButton,
		 ),
	 )
	 verificationButtons.Hide() // Hide initially until a source is selected

	outputScroll := container.NewScroll(outputContainer)
	outputScroll.SetMinSize(fyne.NewSize(800, 500)) // <<< Slightly increased height

	rightPanel := container.NewBorder(
		container.NewVBox(batchLabel, widget.NewSeparator()), // Top: Batch Label
		container.NewVBox(loadMoreButton, layout.NewSpacer(), verificationButtons), // Bottom: Load More & Verification
		 nil, // Left
		 nil, // Right
		 outputScroll, // Center: Address list
	)

	// --- Main Layout ---
	mainContent := container.NewHSplit(
		container.NewScroll(leftPanel),
		rightPanel,
	)
	mainContent.Offset = 0.35 // <<< Adjusted split ratio

	myWindow.SetContent(mainContent)
	myWindow.Resize(fyne.NewSize(1250, 750)) // <<< Increased default size
	myWindow.ShowAndRun()
}

// --- Status Update Functions ---
func showStatus(msg string, isError bool) {
	log.Println("Status:", msg)
	 if statusBinding != nil {
		 err := statusBinding.Set(msg)
		 if err != nil {
			 log.Printf("WARN: Failed to set status binding: %v", err)
		 }
	 } else {
		 log.Println("WARN: statusBinding is nil")
	 }
	 // Optionally change status label color for errors - Fyne doesn't directly support this easily for Label
}

func clearStatus() {
	 if statusBinding != nil {
		 err := statusBinding.Set("")
		 if err != nil {
			 log.Printf("WARN: Failed to clear status binding: %v", err)
		 }
	 } else {
		 log.Println("WARN: statusBinding is nil")
	 }
}

// --- Core Logic Functions (generateNewSeed, decodeMnemonic, loadNextBatch) ---
// ... (No changes needed in these functions for visual improvements)

// generateNewSeedAndAddresses handles the logic for generating a new seed and the first batch of addresses.
func generateNewSeedAndAddresses() {
	passphrase := []byte(passphraseEntry.Text)
	 if len(passphrase) == 0 {
		 passphrase = []byte("aezeed")
		 log.Println("Usando passphrase padrão 'aezeed'")
	 }

	 var entropy [crypto.EntropySize]byte
	 if _, err := rand.Read(entropy[:]); err != nil {
		 errMsg := fmt.Sprintf("Erro ao gerar entropia: %v", err)
		 showStatus(errMsg, true)
		 updateXPUBDisplay()
		 return
	 }

	 birthTime := time.Now()
	 seed, err := crypto.New(0, &entropy, birthTime)
	 if err != nil {
		 errMsg := fmt.Sprintf("Erro ao criar nova seed: %v", err)
		 showStatus(errMsg, true)
		 updateXPUBDisplay()
		 return
	 }

	 mnemonicArray, err := seed.ToMnemonic(passphrase)
	 if err != nil {
		 errMsg := fmt.Sprintf("Erro ao gerar mnemônico: %v", err)
		 showStatus(errMsg, true)
		 updateXPUBDisplay()
		 return
	 }

	 mnemonicEntry.SetText(strings.Join(mnemonicArray[:], " "))

	 masterKey, err := hdkeychain.NewMaster(seed.Entropy[:], netParams)
	 if err != nil {
		 errMsg := fmt.Sprintf("Erro ao derivar chave mestra: %v", err)
		 showStatus(errMsg, true)
		 updateXPUBDisplay()
		 return
	 }
	 currentMasterKey = masterKey
	 currentBatchStart = 0

	 updateXPUBDisplay()
	 updateAddressGrid()
	 loadMoreButton.Enable()
	 // Enable verification buttons if a source other than Offline is selected
	 if selectedBlockchainSource != SourceOffline {
		 verificationButtons.Show()
	 }
	 showStatus("Nova seed e mnemônico gerados com sucesso!", false)
}

// decodeMnemonicAndAddresses handles the logic for decoding a mnemonic and generating the first batch of addresses.
func decodeMnemonicAndAddresses() {
	mnemonicStr := mnemonicEntry.Text
	passphrase := []byte(passphraseEntry.Text)
	 if len(passphrase) == 0 {
		 passphrase = []byte("aezeed")
		 log.Println("Usando passphrase padrão 'aezeed'")
	 }

	 words := strings.Fields(mnemonicStr)
	 if len(words) != crypto.NumMnemonicWords {
		 errMsg := fmt.Sprintf("Erro: Mnemônico deve ter %d palavras, mas tem %d", crypto.NumMnemonicWords, len(words))
		 showStatus(errMsg, true)
		 updateXPUBDisplay()
		 return
	 }

	 var mnemonic crypto.Mnemonic
	 copy(mnemonic[:], words)

	 seed, err := mnemonic.ToCipherSeed(passphrase)
	 if err != nil {
		 errMsg := fmt.Sprintf("Erro ao decodificar mnemônico (verifique palavras e passphrase): %v", err)
		 showStatus(errMsg, true)
		 updateXPUBDisplay()
		 return
	 }

	 masterKey, err := hdkeychain.NewMaster(seed.Entropy[:], netParams)
	 if err != nil {
		 errMsg := fmt.Sprintf("Erro ao derivar chave mestra da seed decodificada: %v", err)
		 showStatus(errMsg, true)
		 updateXPUBDisplay()
		 return
	 }
	 currentMasterKey = masterKey
	 currentBatchStart = 0

	 updateXPUBDisplay()
	 updateAddressGrid()
	 loadMoreButton.Enable()
	 // Enable verification buttons if a source other than Offline is selected
	 if selectedBlockchainSource != SourceOffline {
		 verificationButtons.Show()
	 }
	 showStatus("Mnemônico decodificado com sucesso!", false)
}

// loadNextBatch loads the next batch of addresses.
func loadNextBatch() {
	 if currentMasterKey == nil {
		 showStatus("Erro: Nenhuma chave mestra disponível. Gere ou decodifique uma seed primeiro.", true)
		 return
	 }
	 currentBatchStart += AddressBatchSize
	 updateAddressGrid()
	 showStatus(fmt.Sprintf("Carregado lote de endereços a partir do índice %d", currentBatchStart), false)
}

// --- UI Update Functions (updateXPUBDisplay, updateAddressGrid) ---

// <<< Added copy buttons to XPUBs
func updateXPUBDisplay() {
	 if currentMasterKey == nil {
		 xpubContainer.Objects = []fyne.CanvasObject{
			 widget.NewLabelWithStyle(fmt.Sprintf("Chaves Públicas Estendidas (XPUBs) da Conta %d:", DefaultAccount), fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			 widget.NewLabel("Erro - Chave mestra não disponível."),
		 }
		 xpubContainer.Refresh()
		 return
	 }

	 xpubs := []fyne.CanvasObject{
		 widget.NewLabelWithStyle(fmt.Sprintf("Chaves Públicas Estendidas (XPUBs) da Conta %d:", DefaultAccount), fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
	 }
	 purposes := map[string]uint32{
		 fmt.Sprintf("BIP44 (Legacy) m/44'/0'/%d'", DefaultAccount): BIP44Purpose,
		 fmt.Sprintf("BIP49 (Nested SegWit) m/49'/0'/%d'", DefaultAccount): BIP49Purpose,
		 fmt.Sprintf("BIP84 (Native SegWit) m/84'/0'/%d'", DefaultAccount): BIP84Purpose,
		 fmt.Sprintf("BIP86 (Taproot) m/86'/0'/%d'", DefaultAccount): BIP86Purpose,
	 }

	 for path, purpose := range purposes {
		 xpubStr, err := deriveAccountXpub(currentMasterKey, purpose, CoinTypeBitcoin, DefaultAccount, netParams)
		 var displayLabel *widget.Label
		 var copyButton *widget.Button

		 if err != nil {
			 displayStr := fmt.Sprintf("%s: Erro ao derivar - %v", path, err)
			 displayLabel = widget.NewLabel(displayStr)
			 showStatus(displayStr, true)
			 copyButton = widget.NewButtonWithIcon("", theme.ContentCopyIcon(), func() {})
			 copyButton.Disable()
		 } else {
			 displayLabel = widget.NewLabel(fmt.Sprintf("%s:", path))
			 xpubValue := xpubStr // Capture value for closure
			 copyButton = widget.NewButtonWithIcon("", theme.ContentCopyIcon(), func() {
				 mainWindow.Clipboard().SetContent(xpubValue)
				 showStatus(fmt.Sprintf("XPUB %s copiado!", path), false)
			 })
			 // Add the XPUB itself as a separate, potentially wrapping label or entry
			 xpubEntry := widget.NewMultiLineEntry()
			 xpubEntry.SetText(xpubValue)
			 xpubEntry.Wrapping = fyne.TextWrapBreak
			 xpubEntry.Disable()
			xpubs = append(xpubs, container.NewBorder(nil, nil, displayLabel, copyButton, xpubEntry))
			continue // Skip appending the simple HBox below
		 }
		 // Fallback for error case (label + disabled button)
		 xpubs = append(xpubs, container.NewHBox(displayLabel, copyButton))
	 }

    // Adiciona a Master Fingerprint no início da lista de xpubs
    if currentMasterKey != nil {
        pubKey, err := currentMasterKey.ECPubKey()
        if err == nil {
            fingerprint := btcutil.Hash160(pubKey.SerializeCompressed())[:4]
            fingerprintHex := fmt.Sprintf("%x", fingerprint)
            mfLabel := widget.NewLabelWithStyle(fmt.Sprintf("Master Fingerprint: %s", fingerprintHex), fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
            // Cria um HBox para o label e um botão de copiar
            mfCopyButton := widget.NewButtonWithIcon("", theme.ContentCopyIcon(), func() {
                mainWindow.Clipboard().SetContent(fingerprintHex)
                showStatus(fmt.Sprintf("Master Fingerprint %s copiada!", fingerprintHex), false)
            })
            mfContainer := container.NewBorder(nil, nil, mfLabel, mfCopyButton, widget.NewLabel("")) // Label vazio para empurrar o botão para a direita

            // Adiciona um separador antes das XPUBs, se já houver XPUBs
            if len(xpubs) > 1 { // >1 porque o primeiro é o título da seção
                 xpubs = append([]fyne.CanvasObject{mfContainer, widget.NewSeparator()}, xpubs...)
            } else {
                 xpubs = append([]fyne.CanvasObject{mfContainer}, xpubs...)
            }
        } else {
            showStatus("Erro ao obter chave pública para Master Fingerprint", true)
        }
    }


    xpubContainer.Objects = xpubs
    xpubContainer.Refresh()
}

// <<< Changed address display to Label + Copy Button
func updateAddressGrid() {
	 if currentMasterKey == nil {
		 outputContainer.Objects = []fyne.CanvasObject{widget.NewLabel("Gere ou decodifique uma seed para ver os endereços.")}
		 outputContainer.Refresh()
		 return
	 }

	 batchLabel.SetText(fmt.Sprintf("Endereços (Índices %d-%d, Change %d):", currentBatchStart, currentBatchStart+AddressBatchSize-1, currentChangeType))

	 grid := container.NewGridWithColumns(5)
	 grid.Add(widget.NewLabelWithStyle("Índice", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}))
	 grid.Add(widget.NewLabelWithStyle("Legado (P2PKH)", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}))
	 grid.Add(widget.NewLabelWithStyle("Nested SegWit", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}))
	 grid.Add(widget.NewLabelWithStyle("SegWit Nativo", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}))
	 grid.Add(widget.NewLabelWithStyle("Taproot (P2TR)", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}))

	 for i := uint32(0); i < AddressBatchSize; i++ {
		 index := currentBatchStart + i

		 legacyKey, errL := deriveChildKey(currentMasterKey, BIP44Purpose, CoinTypeBitcoin, DefaultAccount, currentChangeType, index)
		 nestedKey, errN := deriveChildKey(currentMasterKey, BIP49Purpose, CoinTypeBitcoin, DefaultAccount, currentChangeType, index)
		 nativeKey, errNa := deriveChildKey(currentMasterKey, BIP84Purpose, CoinTypeBitcoin, DefaultAccount, currentChangeType, index)
		 taprootKey, errT := deriveChildKey(currentMasterKey, BIP86Purpose, CoinTypeBitcoin, DefaultAccount, currentChangeType, index)

		 grid.Add(widget.NewLabel(strconv.FormatUint(uint64(index), 10)))

		 // Helper function to create label + copy button HBox
		 createAddressCell := func(key *hdkeychain.ExtendedKey, errKey error, genFunc func(*hdkeychain.ExtendedKey, *chaincfg.Params) (btcutil.Address, error)) fyne.CanvasObject {
			 if errKey != nil {
				 return widget.NewLabel("Erro Deriv.")
			 }
			 addr, errGen := genFunc(key, netParams)
			 if errGen != nil {
				 return widget.NewLabel("Erro Gen.")
			 }
			 addrStr := addr.String()
			 addrLabel := widget.NewLabel(addrStr)
			 copyBtn := widget.NewButtonWithIcon("", theme.ContentCopyIcon(), func() {
				 mainWindow.Clipboard().SetContent(addrStr)
				 showStatus(fmt.Sprintf("Endereço %s copiado!", addrStr), false)
			 })
			 // Use Border layout to keep button small and label expandable
			 return container.NewBorder(nil, nil, nil, copyBtn, addrLabel)
		 }

		 grid.Add(createAddressCell(legacyKey, errL, generateLegacyAddress))
		 grid.Add(createAddressCell(nestedKey, errN, generateNestedSegWitAddress))
		 grid.Add(createAddressCell(nativeKey, errNa, generateNativeSegWitAddress))
		 grid.Add(createAddressCell(taprootKey, errT, generateTaprootAddress))
	 } // <<< FECHAMENTO DO LOOP FOR ADICIONADO AQUI

	    if currentBatchStart == 0 { // Primeiro lote sendo carregado
        // Limpa a mensagem inicial ou os grids de uma seed anterior
        outputContainer.Objects = []fyne.CanvasObject{}
    }
    outputContainer.Add(grid) // Adiciona o novo grid ao container existente
    outputContainer.Refresh()
}

// --- Action Handlers (checkDerivationInfo, handleAddressLookup) ---

// <<< Refined button disabling logic
func checkDerivationInfo(purpose uint32, purposeName string) {
	 if selectedBlockchainSource == SourceOffline {
		 showStatus("Verificação desabilitada no modo Offline.", false)
		 return
	 }
	 if currentMasterKey == nil {
		 showStatus("Erro: Nenhuma chave mestra disponível. Gere ou decodifique uma seed primeiro.", true)
		 return
	 }

	 clearStatus()
	 showStatus(fmt.Sprintf("Iniciando verificação para %d endereços %s via %s...", AddressBatchSize, purposeName, selectedBlockchainSource), false)

	 // Disable relevant buttons and show progress
	 progressBar.Show()
	 generateButton.Disable()
	 decodeButton.Disable()
	 loadMoreButton.Disable()
	 addressLookupButton.Disable()
	 verifyLegacyButton.Disable()
	 verifyNestedButton.Disable()
	 verifyNativeButton.Disable()
	 verifyTaprootButton.Disable()
	 // Keep accountToggleButton enabled if desired, or disable too
	 // accountToggleButton.Disable()

	 defer func() {
		 progressBar.Hide()
		 generateButton.Enable()
		 decodeButton.Enable()
		 if currentMasterKey != nil { // Only enable if key is still valid
			 loadMoreButton.Enable()
			 addressLookupButton.Enable()
			 verifyLegacyButton.Enable()
			 verifyNestedButton.Enable()
			 verifyNativeButton.Enable()
			 verifyTaprootButton.Enable()
			 // accountToggleButton.Enable()
		 }
	 }()

	 results := make([]string, AddressBatchSize)
	 errors := make([]error, AddressBatchSize)
	 addresses := make([]btcutil.Address, AddressBatchSize)
	 keys := make([]*hdkeychain.ExtendedKey, AddressBatchSize)

	 // First pass: Derive keys and addresses
	 derivationErrors := false
	 for i := uint32(0); i < AddressBatchSize; i++ {
		 index := currentBatchStart + i
		 key, err := deriveChildKey(currentMasterKey, purpose, CoinTypeBitcoin, DefaultAccount, currentChangeType, index)
		 if err != nil {
			 errors[i] = fmt.Errorf("idx %d: erro ao derivar chave: %w", index, err)
			 derivationErrors = true
			 continue
		 }
		 keys[i] = key

		 var addr btcutil.Address
		 switch purpose {
		 case BIP44Purpose: addr, err = generateLegacyAddress(key, netParams)
		 case BIP49Purpose: addr, err = generateNestedSegWitAddress(key, netParams)
		 case BIP84Purpose: addr, err = generateNativeSegWitAddress(key, netParams)
		 case BIP86Purpose: addr, err = generateTaprootAddress(key, netParams)
		 default:
			 errors[i] = fmt.Errorf("idx %d: propósito desconhecido %d", index, purpose)
			 derivationErrors = true
			 continue
		 }

		 if err != nil {
			 errors[i] = fmt.Errorf("idx %d: erro ao gerar endereço: %w", index, err)
			 derivationErrors = true
		 } else {
			 addresses[i] = addr
		 }
	 }

	 if derivationErrors {
		 showStatus("Erros ocorreram durante a derivação de chaves/endereços. Verificação online não iniciada.", true)
		 // Display only derivation errors (code omitted for brevity, same as before)
		 return
	 }

	 // Second pass: Perform online checks
	 if selectedBlockchainSource == SourceLocalNode {
		 log.Println("Iniciando verificação sequencial via Nó Local...")
		 for i := uint32(0); i < AddressBatchSize; i++ {
			 addr := addresses[i]
			 key := keys[i]
			 addrStr := addr.String()
			 index := currentBatchStart + i
			 showStatus(fmt.Sprintf("Verificando Nó Local para índice %d (%s)...", index, addrStr), false)
			 time.Sleep(500 * time.Millisecond)
			 info, err := checkAddressLocalNodeWithScan(addr, key, purpose)
			 if err != nil {
				 errors[i] = fmt.Errorf("idx %d (%s): erro na verificação: %w", index, addrStr, err)
			 } else {
				 results[i] = info
			 }
		 }
		 log.Println("Verificação sequencial via Nó Local concluída.")
	 } else {
		 log.Println("Iniciando verificação paralela via Blockstream...")
		 var wg sync.WaitGroup
		 for i := uint32(0); i < AddressBatchSize; i++ {
			 wg.Add(1)
			 go func(idx uint32) {
				 defer wg.Done()
				 addr := addresses[idx]
				 addrStr := addr.String()
				 time.Sleep(apiCallDelay)
				 info, err := checkAddressBlockstream(addrStr)
				 if err != nil {
					 errors[idx] = fmt.Errorf("idx %d (%s): erro na verificação: %w", currentBatchStart+idx, addrStr, err)
				 } else {
					 results[idx] = info
				 }
			 }(i)
		 }
		 wg.Wait()
		 log.Println("Verificação paralela via Blockstream concluída.")
	 }

	 // Process and display results (code omitted for brevity, same as before)
	 var resultBuilder strings.Builder
	 errorCount := 0
	 resultBuilder.WriteString(fmt.Sprintf("Resultados da Verificação para %s (Fonte: %s):\n\n", purposeName, selectedBlockchainSource))
	 for i := uint32(0); i < AddressBatchSize; i++ {
		 index := currentBatchStart + i
		 addrStr := ""
		 if addresses[i] != nil {
			 addrStr = addresses[i].String()
		 }
		 if errors[i] != nil {
			 resultBuilder.WriteString(fmt.Sprintf("Índice %d: Erro - %v\n", index, errors[i]))
			 errorCount++
		 } else if results[i] != "" {
			 resultBuilder.WriteString(fmt.Sprintf("Índice %d (%s): %s\n", index, addrStr, results[i]))
		 } else {
			 resultBuilder.WriteString(fmt.Sprintf("Índice %d (%s): Nenhuma informação retornada.\n", index, addrStr))
		 }
	 }
	 if errorCount > 0 {
		 resultBuilder.WriteString(fmt.Sprintf("\n%d erros ocorreram durante a verificação.", errorCount))
	 }
	 resultEntry := widget.NewMultiLineEntry()
	 resultEntry.SetText(resultBuilder.String())
	 resultEntry.Wrapping = fyne.TextWrapOff
	 resultEntry.Disable()
	 resultScroll := container.NewScroll(resultEntry)
	 resultScroll.SetMinSize(fyne.NewSize(600, 400))
	 dialog.ShowCustom(fmt.Sprintf("Verificação %s Concluída", purposeName), "Fechar", resultScroll, mainWindow)
	 showStatus(fmt.Sprintf("Verificação %s concluída. %d erros.", purposeName, errorCount), errorCount > 0)
}

// checkAddressLocalNodeWithScan uses scantxoutset to find the balance of a specific address.
// ... (No changes needed in this function for visual improvements)
func checkAddressLocalNodeWithScan(address btcutil.Address, key *hdkeychain.ExtendedKey, purpose uint32) (string, error) {
	 client, err := getRPCClient()
	 if err != nil {
		 return "", fmt.Errorf("falha ao obter cliente RPC: %w", err)
	 }
	 descriptor := fmt.Sprintf("addr(%s)", address.String())

	 // Abort existing scan logic (code omitted for brevity, same as before)
	 statusParams := []json.RawMessage{json.RawMessage(`"status"`)}
	 statusBytes, statusErr := client.RawRequest("scantxoutset", statusParams)
	 if statusErr == nil {
		 var scanStatus struct { Progress *float64 `json:"progress"` }
		 if err := json.Unmarshal(statusBytes, &scanStatus); err == nil && scanStatus.Progress != nil {
			 log.Printf("Scan existente em progresso (progresso: %v). Abortando...", *scanStatus.Progress)
			 abortParams := []json.RawMessage{json.RawMessage(`"abort"`)}
			 abortBytes, abortErr := client.RawRequest("scantxoutset", abortParams)
			 if abortErr != nil {
				 log.Printf("Aviso: Erro ao abortar scantxoutset existente: %v", abortErr)
			 } else {
				 var abortResult struct { Success bool `json:"success"` }
				 if err := json.Unmarshal(abortBytes, &abortResult); err == nil && abortResult.Success {
					 log.Println("Scan existente abortado com sucesso.")
					 time.Sleep(200 * time.Millisecond)
				 } else {
					 log.Printf("Falha ao abortar scan existente: %s", string(abortBytes))
				 }
			 }
		 }
	 } else {
		 log.Printf("Aviso: Erro ao verificar status scantxoutset: %v", statusErr)
	 }

	 // Start new scan
	 scanObject := map[string]interface{}{"desc": descriptor}
	 startParams := []json.RawMessage{
		 json.RawMessage(`"start"`),
		 json.RawMessage(fmt.Sprintf(`[%s]`, marshalJSON(scanObject))),
	 }
	 resultBytes, err := client.RawRequest("scantxoutset", startParams)
	 if err != nil {
		 log.Printf("Erro ao chamar scantxoutset RPC para descritor '%s': %v", descriptor, err)
		 if jsonErr, ok := err.(*btcjson.RPCError); ok {
			 log.Printf("Erro RPC específico: Code=%d, Message=%s", jsonErr.Code, jsonErr.Message)
			 if strings.Contains(jsonErr.Message, "requires address index") {
				 return "", fmt.Errorf("erro RPC: scantxoutset requer 'addressindex=1' habilitado no nó Bitcoin Core.")
			 }
			 if strings.Contains(jsonErr.Message, "Scan already in progress") {
				 return "", fmt.Errorf("erro RPC: Scan já em progresso (inesperado)")
			 }
			 return "", fmt.Errorf("erro RPC do nó: %s (Code: %d)", jsonErr.Message, jsonErr.Code)
		 }
		 return "", fmt.Errorf("erro não-RPC ao chamar scantxoutset: %w", err)
	 }
	 var scanResult struct {
		 Success      bool    `json:"success"`
		 TotalAmount  float64 `json:"total_amount"`
	 }
	 err = json.Unmarshal(resultBytes, &scanResult)
	 if err != nil {
		 return "", fmt.Errorf("erro ao decodificar resultado scantxoutset: %w", err)
	 }
	 if !scanResult.Success {
		 return "(Falha no Scan)", nil
	 }
	 return fmt.Sprintf("Saldo: %.8f BTC", scanResult.TotalAmount), nil
}

// --- Address Lookup Logic (findAddressInSeed, handleAddressLookup) ---

// AddressLookupResult holds the result of the address lookup
type AddressLookupResult struct {
	Found     bool
	Purpose   uint32
	Index     uint32
	DerivationPath string
	Address   btcutil.Address
	DerivedKey *hdkeychain.ExtendedKey
}

// findAddressInSeed attempts to find the given address by deriving from the current master key.
// ... (No changes needed in this function for visual improvements)
func findAddressInSeed(targetAddrStr string) (*AddressLookupResult, error) {
	 if currentMasterKey == nil {
		 return nil, fmt.Errorf("nenhuma seed Aezeed carregada")
	 }
	 targetAddr, err := btcutil.DecodeAddress(targetAddrStr, netParams)
	 if err != nil {
		 return nil, fmt.Errorf("endereço Bitcoin inválido: %w", err)
	 }
	 targetAddrStr = targetAddr.String()

	 purposes := map[uint32]string{
		 BIP44Purpose: "BIP44 (Legacy)",
		 BIP49Purpose: "BIP49 (Nested SegWit)",
		 BIP84Purpose: "BIP84 (Native SegWit)",
		 BIP86Purpose: "BIP86 (Taproot)",
	 }

	 log.Printf("Iniciando busca pelo endereço %s até índice %d (change 0 e 1)...", targetAddrStr, addressSearchLimit-1)

	 for purpose, purposeName := range purposes {
		 log.Printf("Verificando derivação %s...", purposeName)
		 for _, changeType := range []uint32{ExternalChain, InternalChain} {
			 log.Printf("  Verificando change %d...", changeType)
			 derivationPrefix := fmt.Sprintf("m/%d'/0'/%d'/%d", purpose, CoinTypeBitcoin, DefaultAccount, changeType)
			 for index := uint32(0); index < addressSearchLimit; index++ {
				 key, err := deriveChildKey(currentMasterKey, purpose, CoinTypeBitcoin, DefaultAccount, changeType, index)
				 if err != nil {
					 if index == 0 && changeType == ExternalChain {
						 log.Printf("Erro ao derivar chave para %s/%d/%d (outros erros omitidos): %v", purposeName, changeType, index, err)
					 }
					 continue
				 }

				 var generatedAddr btcutil.Address
				 var genErr error
				 switch purpose {
				 case BIP44Purpose: generatedAddr, genErr = generateLegacyAddress(key, netParams)
				 case BIP49Purpose: generatedAddr, genErr = generateNestedSegWitAddress(key, netParams)
				 case BIP84Purpose: generatedAddr, genErr = generateNativeSegWitAddress(key, netParams)
				 case BIP86Purpose: generatedAddr, genErr = generateTaprootAddress(key, netParams)
				 default: continue
				 }

				 if genErr != nil {
					 if index == 0 && changeType == ExternalChain {
						 log.Printf("Erro ao gerar endereço para %s/%d/%d (outros erros omitidos): %v", purposeName, changeType, index, genErr)
					 }
					 continue
				 }

				 if generatedAddr.String() == targetAddrStr {
					 log.Printf("Endereço encontrado! Derivação: %s, Change: %d, Índice: %d", purposeName, changeType, index)
					 derivationPathStr := fmt.Sprintf("%s/%d", derivationPrefix, index)
					 return &AddressLookupResult{
						 Found:     true,
						 Purpose:   purpose,
						 Index:     index,
						 DerivationPath: derivationPathStr,
						 Address:   generatedAddr,
						 DerivedKey: key,
					 }, nil
				 }
			 }
		 }
	 }

	 log.Printf("Endereço %s não encontrado na seed atual dentro do limite de busca (%d) para change 0 e 1.", targetAddrStr, addressSearchLimit)
	 return &AddressLookupResult{Found: false}, nil
}

// <<< Refined button disabling logic
func handleAddressLookup() {
	 targetAddrStr := addressLookupEntry.Text
	 if targetAddrStr == "" {
		 showStatus("Por favor, insira um endereço Bitcoin para buscar.", true)
		 return
	 }
	 if currentMasterKey == nil {
		 showStatus("Erro: Nenhuma seed Aezeed carregada. Gere ou decodifique uma seed primeiro.", true)
		 return
	 }

	 clearStatus()
	 showStatus(fmt.Sprintf("Buscando endereço %s na seed atual...", targetAddrStr), false)

	 // Disable relevant buttons and show progress
	 progressBar.Show()
	 generateButton.Disable()
	 decodeButton.Disable()
	 loadMoreButton.Disable()
	 addressLookupButton.Disable()
	 verifyLegacyButton.Disable()
	 verifyNestedButton.Disable()
	 verifyNativeButton.Disable()
	 verifyTaprootButton.Disable()
	 // accountToggleButton.Disable()
			 defer func() { // Re-enable buttons on main thread
				 fyne.Do(func() {
					 progressBar.Hide()
					 generateButton.Enable()
					 decodeButton.Enable()
					 if currentMasterKey != nil { // Only enable if key is still valid
						 loadMoreButton.Enable()
						 addressLookupButton.Enable()
						 verifyLegacyButton.Enable()
						 verifyNestedButton.Enable()
						 verifyNativeButton.Enable()
						 verifyTaprootButton.Enable()
						 // accountToggleButton.Enable()
					 }
				 })
			 }() // Execute the deferred function

		 // 1. Find if address belongs to the seed
		 findResult, findErr := findAddressInSeed(targetAddrStr)

		 // Prepare dialog content
		 var dialogContent strings.Builder
		 dialogContent.WriteString(fmt.Sprintf("Resultado da Busca por: %s\n\n", targetAddrStr))

		 if findErr != nil {
			 dialogContent.WriteString(fmt.Sprintf("Erro na busca: %v", findErr))
			 showStatus(fmt.Sprintf("Erro na busca: %v", findErr), true)
		 } else if !findResult.Found {
			 dialogContent.WriteString(fmt.Sprintf("Resultado: Endereço NÃO encontrado na seed atual (limite de busca: %d por derivação).\n", addressSearchLimit))
			 // Optionally try checking online if not found locally
			 if selectedBlockchainSource != SourceOffline {
				 dialogContent.WriteString(fmt.Sprintf("\nVerificando online via %s...\n", selectedBlockchainSource))
				 var onlineInfo string
				 var onlineErr error
				 if selectedBlockchainSource == SourceBlockstream {
					 onlineInfo, onlineErr = checkAddressBlockstream(targetAddrStr)
				 } else { // SourceLocalNode
					 // Cannot check local node without the derived key/purpose
					 onlineInfo = "(Verificação de saldo via Nó Local requer que o endereço seja encontrado na seed primeiro)"
				 }
				 if onlineErr != nil {
					 dialogContent.WriteString(fmt.Sprintf("Erro na verificação online: %v", onlineErr))
				 } else {
					 dialogContent.WriteString(fmt.Sprintf("Info Online: %s", onlineInfo))
				 }
			 }
			 showStatus("Busca concluída: Endereço não encontrado na seed.", false)
		 } else {
			 // Address FOUND in seed
			 dialogContent.WriteString(fmt.Sprintf("Resultado: Endereço ENCONTRADO!\n"))
			 dialogContent.WriteString(fmt.Sprintf("  Derivação: %s\n", findResult.DerivationPath))
			 // Optionally check balance online if found
			 if selectedBlockchainSource != SourceOffline {
				 dialogContent.WriteString(fmt.Sprintf("\nVerificando online via %s...\n", selectedBlockchainSource))
				 var onlineInfo string
				 var onlineErr error
				 if selectedBlockchainSource == SourceBlockstream {
					 onlineInfo, onlineErr = checkAddressBlockstream(targetAddrStr)
				 } else { // SourceLocalNode
					 // Now we have the key and purpose
					 onlineInfo, onlineErr = checkAddressLocalNodeWithScan(findResult.Address, findResult.DerivedKey, findResult.Purpose)
				 }
				 if onlineErr != nil {
					 dialogContent.WriteString(fmt.Sprintf("Erro na verificação online: %v", onlineErr))
				 } else {
					 dialogContent.WriteString(fmt.Sprintf("Info Online: %s", onlineInfo))
				 }
			 }
				 showStatus("Busca concluída: Endereço encontrado na seed!", false) // <<< Removed stray backslash
			 } // End of else block (Address Found)

		 // Show result in dialog (on main thread)
		 fyne.Do(func() {
			 dialog.ShowInformation("Resultado da Busca", dialogContent.String(), mainWindow)
		 })

}
// End of handleAddressLookup function

// Helper function needed by the crypto package
func timeFromBitcoinDaysGenesis(days uint16) time.Time {
	dayDuration := time.Duration(days) * 24 * time.Hour
	return crypto.BitcoinGenesisDate.Add(dayDuration)
}

