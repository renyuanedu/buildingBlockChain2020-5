package main

import (
	"fmt"
	"bytes"
	"encoding/gob"
	"log"
	"io/ioutil"
	"crypto/elliptic"
	"os"
)

const walletFile = "wallet.dat"
//多个钱包账户管理
type Wallets struct{

	Walletsstore map[string]*Wallet
}
//新建钱包s
func NewWallets() (*Wallets,error){
	wallets := Wallets{}

	wallets.Walletsstore =  make(map[string]*Wallet)

	err:= wallets.LoadFromFile()

	return &wallets,err


}
//创建钱包并且加入钱包s
func (ws *Wallets) CreateWallet() string{

	wallet := Newwallet()

	address := fmt.Sprintf("%s",wallet.GetAddress())
	ws.Walletsstore[address] = wallet

	return address
}
//根据地址获取对应钱包
func (ws * Wallets) GetWallet(address string) Wallet{
	return *ws.Walletsstore[address]
}
//获取钱包s里面的所有地址
func (ws * Wallets) getAddress() []string{

	var addresses []string

	for address,_ := range ws.Walletsstore{

		addresses =  append(addresses,address)
	}


	return addresses
}

//反序列化读取钱包数据
func (ws * Wallets) LoadFromFile() error{
	if _ ,err :=  os.Stat(walletFile);os.IsNotExist(err){
		return err
	}

		fileContent,err:=ioutil.ReadFile(walletFile)

		if err !=nil{
			log.Panic(err)
		}

		var wallets Wallets
		gob.Register(elliptic.P256())
		decoder := gob.NewDecoder(bytes.NewReader(fileContent))
		err = decoder.Decode(&wallets)

		if err !=nil{
			log.Panic(err)
		}

		ws.Walletsstore =  wallets.Walletsstore

		return nil
}

//钱包数据序列化入库
func (ws *Wallets) SaveToFile(){

	var content bytes.Buffer

	gob.Register(elliptic.P256())
	encoder:= gob.NewEncoder(&content)

	err := encoder.Encode(ws)
	if err !=nil{

		log.Panic(err)
	}

	err = ioutil.WriteFile(walletFile,content.Bytes(),0777)
	if err !=nil{

		log.Panic(err)
	}
}