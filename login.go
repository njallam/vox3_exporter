package main

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"io/ioutil"
	"log"
	"math/big"
	"net/url"
)

var gen = big.NewInt(2)
var k, _ = big.NewInt(0).SetString("ac6bdb41324a9a9bf166de5e1389582faf72b6651987ee07fc3192943db56050a37329cbb4a099ed8193e0757767a13dd52312ab4b03310dcd7f48a9da04fd50e8083969edb767b0cf6095179a163ab3661a05fbd5faaae82918a9962f0b93b855f97993ec975eeaa80d740adbf4ff747359d041d5c33ea71d281e446b14773bca97b43a23fb801676bd207a436c6481f1d2b9078717461a5b9d32e688f87748544523b524b0d57d5ea77a2775d2ecfa032cfbdbf52fb3786160279004e57ae6af874e7303ce53299ccc041c7bc308d82a5698f3a8d0c38271ae35f8e9dbfbb694b5c803d89f7ae435de236d525f54759b65e372fcd68ef20fa7111f9e4aff73", 16)
var C, _ = big.NewInt(0).SetString("05b9e8ef059c6b32ea59fc1d322d37f04aa30bae5aa9003b8321e21ddb04e300", 16)

const u = "4a76a9a2402bdd18123389b72ebbda50a30f65aedb90d7273130edea4b29cc4c"

func (collector *Vox3Collector) login(token string) {
	log.Println("Logging in")
	randBytes := make([]byte, 8)
	_, err := io.ReadFull(rand.Reader, randBytes)
	if err != nil {
		panic("Random source is broken!")
	}
	F := big.NewInt(0).SetBytes(randBytes)

	D := big.NewInt(0)
	D.Exp(gen, F, k)
	x := D.Text(16)

	response, err := collector.client.PostForm(collector.baseURL+"/authenticate", url.Values{
		"CSRFtoken": {token},
		"I":         {"vodafone"},
		"A":         {x},
	})
	if err != nil {
		log.Fatal("Error loading HTTP response body", err)
	}
	var body struct {
		S string `json:"s"`
		B string `json:"B"`
	}
	jsonData, _ := ioutil.ReadAll(response.Body)
	json.Unmarshal(jsonData, &body)

	g, _ := big.NewInt(0).SetString(body.B, 16)

	q := 256
	var dBytes = D.Bytes()
	if len(dBytes) > q {
		dBytes = dBytes[1:]
	}

	var gBytes = g.Bytes()
	if len(gBytes) > q {
		gBytes = gBytes[1:]
	}

	temp := sha256.Sum256(append(dBytes, gBytes...))
	h := big.NewInt(0).SetBytes(temp[:])
	temp = sha256.Sum256([]byte("vodafone:" + collector.password))
	temp2, _ := hex.DecodeString(body.S + hex.EncodeToString(temp[:]))
	temp = sha256.Sum256(temp2)
	n := big.NewInt(0).SetBytes(temp[:])

	a := big.NewInt(0)
	a.Exp(gen, n, k)
	a.Mul(C, a)
	a.Mod(a, k)

	b := big.NewInt(0)
	b.Mul(h, n)
	b.Mod(b, k)
	b.Add(b, F)
	b.Mod(b, k)

	g.Sub(g, a)
	g.Mod(g, k)
	g.Exp(g, b, k)

	e := g.Text(16)
	if len(e)%2 == 1 {
		e = "0" + e
	}

	temp2, _ = hex.DecodeString(e)
	temp = sha256.Sum256(temp2)
	B := hex.EncodeToString(temp[:])
	temp = sha256.Sum256([]byte("vodafone"))
	e = hex.EncodeToString(temp[:])
	temp2, _ = hex.DecodeString(u + e + body.S + x + body.B + B)
	temp = sha256.Sum256(temp2)
	y := hex.EncodeToString(temp[:])

	temp2, _ = hex.DecodeString(x + y + B)
	temp = sha256.Sum256(temp2)
	v := hex.EncodeToString(temp[:])

	response, err = collector.client.PostForm(collector.baseURL+"/authenticate", url.Values{
		"CSRFtoken": {token},
		"M":         {y},
	})
	if err != nil {
		log.Fatal("Error loading HTTP response body", err)
	}
	var body2 struct {
		M string `json:"M"`
	}
	jsonData, _ = ioutil.ReadAll(response.Body)
	json.Unmarshal(jsonData, &body2)

	if v != body2.M {
		log.Println("v != M")
	}
}
