package main

import (
	"bufio"
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdsa"
	"crypto/elliptic"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"math/big"
	"net/http"
	"os"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto/secp256k1"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/seithhq/crypto-finder/usdttoken"
	"golang.org/x/crypto/sha3"
)

const (
	INFURA = "https://mainnet.infura.io/v3/8f55e3b466dc48da85e06b50177c4c0b"
)

func main() {
	wrk := func(_ int, jobs <-chan string, results chan<- string) {
		for private := range jobs {
			result := fmt.Sprintf("%s;0x%s", private, addressFromPrivate(private))
			results <- result
		}
	}

	full := parseCombinations("storage/ruby_input.txt")
	fmt.Println("private_key;address")

	start := 0
	len := len(full)

	addresses := full[start : start+len]

	jobs := make(chan string, len)
	results := make(chan string, len)

	for w := 1; w <= 10; w++ {
		go wrk(w, jobs, results)
	}

	for j := 0; j < len; j++ {
		jobs <- addresses[j]
	}
	close(jobs)

	for a := 0; a < len; a++ {
		fmt.Println(<-results)
	}
}

func parseCombinations(filename string) []string {
	addresses := make([]string, 0)

	file, err := os.Open(filename)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Split(bufio.ScanLines)

	for scanner.Scan() {
		text := scanner.Text()
		addresses = append(addresses, strings.Trim(text, " "))
	}

	return addresses
}

func getBalance(client *ethclient.Client, rawAddress string) string {
	tokenAddress := common.HexToAddress(usdttoken.ADDRESS)
	instance, err := usdttoken.NewUsdttoken(tokenAddress, client)
	if err != nil {
		log.Fatal(err)
	}

	address := common.HexToAddress(rawAddress)
	balance, err := instance.BalanceOf(&bind.CallOpts{}, address)
	if err != nil {
		log.Fatal(err)
	}

	fbalance := new(big.Float)
	fbalance.SetString(balance.String())
	ethValue := new(big.Float).Quo(fbalance, big.NewFloat(math.Pow10(18)))

	return ethValue.String()
}

type MethodResponse struct {
	ID      int    `json:"id"`
	JsonRPC string `json:"jsonrpc"`
	Result  struct {
		Address       string  `json:"address"`
		TokenBalances []Token `json:"tokenBalances"`
	} `json:"result"`
}

type Token struct {
	ContractAddress string `json:"contractAddress"`
	TokenBalance    string `json:"tokenBalance"`
}

type Param struct {
	ID      int      `json:"id"`
	JsonRPC string   `json:"jsonrpc"`
	Method  string   `json:"method"`
	Params  []string `json:"params"`
}

func parseTokens(tokens []Token) string {
	result := make([]string, 0)

	symbolsMap := map[string]string{
		"0x6b175474e89094c44da98b954eedeac495271d0f": "DAI",
		"0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2": "WETH",
		"0xdAC17F958D2ee523a2206206994597C13D831ec7": "USDT",
	}

	emptyBalance := "0x0000000000000000000000000000000000000000000000000000000000000000"

	for _, token := range tokens {
		symbol, ok := symbolsMap[token.ContractAddress]
		if ok && token.TokenBalance != emptyBalance {
			result = append(result, symbol+"#"+token.TokenBalance)
		}
	}

	if len(result) == 0 {
		return "ZER"
	}

	return strings.Join(result, ",")
}

func getTokens(addresses []string) string {
	url := "https://eth-mainnet.g.alchemy.com/v2/alcht_nqnlLRumvopEQ4nLZDQWNyBz41URJW"

	addrs := make([]string, len(addresses))
	for i := 0; i < len(addresses); i++ {
		addrs[i] = "0x" + addresses[i]
	}

	params := Param{
		ID:      1,
		JsonRPC: "2.0",
		Method:  "alchemy_getTokenBalances",
		Params:  addrs,
	}

	payload, err := json.Marshal(&params)
	if err != nil {
		return err.Error()
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(payload))
	if err != nil {
		return err.Error()
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err.Error()
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return err.Error()
	}
	defer res.Body.Close()

	methodResponse := &MethodResponse{}
	err = json.Unmarshal(body, &methodResponse)
	if err != nil {
		return err.Error()
	}

	if len(methodResponse.Result.TokenBalances) == 0 {
		return "ZER"
	}

	return parseTokens(methodResponse.Result.TokenBalances)
}

func BytesToBig(data []byte) *big.Int {
	n := new(big.Int)
	n.SetBytes(data)

	return n
}

func ToECDSA(prv []byte) *ecdsa.PrivateKey {
	priv := new(ecdsa.PrivateKey)
	priv.PublicKey.Curve = secp256k1.S256()
	priv.D = BytesToBig(prv)
	priv.PublicKey.X, priv.PublicKey.Y = secp256k1.S256().ScalarBaseMult(prv)

	return priv
}

func Keccak256(data ...[]byte) []byte {
	d := sha3.NewLegacyKeccak256()
	for _, b := range data {
		d.Write(b)
	}
	return d.Sum(nil)
}

func decryptAES(encryptedString, keyString, nonceString string) (string, error) {
	key, _ := hex.DecodeString(keyString)
	enc, _ := hex.DecodeString(encryptedString)

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := len(nonceString)
	nonce, ciphertext := enc[:nonceSize], enc[nonceSize:]

	plaintext, err := aesGCM.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}

func addressFromPrivate(privateKey string) string {
	hexBuf, _ := hex.DecodeString(privateKey)
	priv := ToECDSA(hexBuf)
	pub := priv.PublicKey

	ecdsaPubBytes := elliptic.Marshal(secp256k1.S256(), pub.X, pub.Y)
	addressBytes := Keccak256(ecdsaPubBytes[1:])[12:]
	return hex.EncodeToString(addressBytes)
}
