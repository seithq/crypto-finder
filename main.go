package main

import (
	"bufio"
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

const INFURA = "https://mainnet.infura.io/v3/8f55e3b466dc48da85e06b50177c4c0b"

func main() {
	// client, err := ethclient.Dial(INFURA)
	// if err != nil {
	// 	log.Fatal(err)
	// }
	// defer client.Close()

	wrk := func(_ int, jobs <-chan string, results chan<- string) {
		for hash := range jobs {

			hexBuf, _ := hex.DecodeString(hash)
			priv := ToECDSA(hexBuf)
			pub := priv.PublicKey

			ecdsaPubBytes := elliptic.Marshal(secp256k1.S256(), pub.X, pub.Y)
			addressBytes := Keccak256(ecdsaPubBytes[1:])[12:]

			address := fmt.Sprintf("0x%s", hex.EncodeToString(addressBytes))
			// result := fmt.Sprintf("%s -> %s -> %s", hash, address, getBalance(client, address))
			result := fmt.Sprintf("%s -> %s -> %s", hash, address, getTokens(address))

			results <- result
		}
	}

	hashes := parseHashes("storage/input.txt")
	len := len(hashes)

	jobs := make(chan string, len)
	results := make(chan string, len)

	for w := 1; w <= 3; w++ {
		go wrk(w, jobs, results)
	}

	for j := 0; j < len; j++ {
		jobs <- hashes[j]
	}
	close(jobs)

	for a := 0; a < len; a++ {
		fmt.Println(<-results)
	}
}

func parseHashes(filename string) []string {
	hashes := make([]string, 0)

	file, err := os.Open(filename)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Split(bufio.ScanLines)

	for scanner.Scan() {
		text := scanner.Text()
		if strings.HasPrefix(text, "---") {
			continue
		}

		parts := strings.Split(text, ":")
		if len(parts) < 2 {
			continue
		}

		hash := strings.Trim(parts[1], " ")
		hashes = append(hashes, hash)
	}

	return hashes
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

type Token struct {
	Symbol string `json:"symbol"`
}

func getTokens(rawAddress string) string {
	url := fmt.Sprintf("https://deep-index.moralis.io/api/v2.2/%s/erc20?chain=eth", rawAddress)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err.Error()
	}

	req.Header.Add("Accept", "application/json")
	req.Header.Add("X-API-Key", "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJub25jZSI6IjgyOGQ3NjhhLTE2MzMtNDRjNC05NmY1LTFjNmZjZWQxNDM4ZiIsIm9yZ0lkIjoiNDEzMjE2IiwidXNlcklkIjoiNDI0NjU0IiwidHlwZUlkIjoiZGI4Y2FlNjktMDNhOC00YjAxLTgyNzUtZTU2OTAwOTY2M2ZlIiwidHlwZSI6IlBST0pFQ1QiLCJpYXQiOjE3Mjk4MDYwODgsImV4cCI6NDg4NTU2NjA4OH0.GFcjwGsXqpDo6KLG-tHhmfajS6rMNR7a_-2ULa51vGU")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err.Error()
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return err.Error()
	}

	tokens := []Token{}
	err = json.Unmarshal(body, &tokens)
	if err != nil {
		return err.Error()
	}

	if len(tokens) == 0 {
		return "NIL"
	}

	out := []string{}
	for _, token := range tokens {
		out = append(out, token.Symbol)
	}

	return strings.Join(out, ",")
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
