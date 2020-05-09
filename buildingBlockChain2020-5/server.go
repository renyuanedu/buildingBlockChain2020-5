package main

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
)

const nodeversion  = 0x00   //版本信息

var nodeAddress string  //节点地址
var blockInTransit [][]byte

const commonLength = 12  //固定命令长度

type Version struct {
	Version int   //版本号
	BestHeight int32   //当前最高高度
	AddrFrom string 	// 源地址
}

//信息打印
func (ver *Version) String(){
	fmt.Printf("Version:%d\n",ver.Version)
	fmt.Printf("BestHeight:%d\n",ver.BestHeight)
	fmt.Printf("AddrFrom:%s\n",ver.AddrFrom)
}


var knownNodes = []string{"localhost:3000"}   //已知节点

//服务开启
func StartServer(nodeID,minerAddress string,bc*Blockchain){

	nodeAddress = fmt.Sprintf("localhost:%s",nodeID)  //当前节点地址
	ln,err:= net.Listen("tcp",nodeAddress)  //监听端口信息
	defer ln.Close()

	//bc := NewBlockchain("17ebTuztFPtct1jV8ocjXXSSRiBkZnbGfe") //新建区块链

	//不等于已知节点，就发送版本号
	if nodeAddress !=knownNodes[0]{
		sendVersion(knownNodes[0],bc)
	}

	for{

		conn,err2:=ln.Accept()
		//持续监听接受信息
		if err2 != nil{
			log.Panic(err)
		}
		go handleConnction(conn,bc)
		//根据信息，创建协程，处理链接
	}
}


func handleConnction(conn net.Conn, bc *Blockchain) {
	//获取请求内容
	request,err := ioutil.ReadAll(conn)

	if err !=nil{
		log.Panic(err)
	}

	//获取命令，判断执行内容
	command:= bytesToCommand(request[:commonLength])
	fmt.Println(command)
	switch command {
	case "version":
		handleVersion(request,bc) //获取版本信息
	case "getblocks": //获取外部区块信息
		handleGetBlock(request,bc)
	case "inv":
		handleInv(request,bc) //区块信息处理
	case "getdata":
		handleGetData(request,bc) //对于获取区块详情请求的处理
	case "block":
		handleBlock(request,bc)
	}
}


// 将获取的区块信息进行反序列化，变成data
func handleBlock(request []byte, bc *Blockchain) {
	var buff bytes.Buffer
	var payload blocksend
	buff.Write(request[commonLength:])
	dec:= gob.NewDecoder(&buff)
	err := dec.Decode(&payload)
	if err !=nil{
		log.Panic(err)
	}
	//获取区块信息，然后反序列化
	blockdata:= payload.Block
	block:= DeserializeBlock(blockdata)
	//把区块增加至链中
	bc.AddBlock(block)
	fmt.Printf("Recieve a new Block")
	//再次判定是否为最新区块
	if len(blockInTransit) >0{
		blockHash:= blockInTransit[0]
		sendGetData(payload.AddrFrom,"block",blockHash)
		//将数据后面的区块更新
		blockInTransit = blockInTransit[1:]
	}else{
		//更新UTXO
		set:= UTXOSet{bc}
		set.Reindex()
	}
}


//获取区块详情
func handleGetData(request []byte, bc *Blockchain) {
	fmt.Println("获取区块详情")
	var buff bytes.Buffer
	var payload getdata
	buff.Write(request[commonLength:])
	dec:=gob.NewDecoder(&buff)
	err:= dec.Decode(&payload)
	if err !=nil{
		log.Panic(err)
	}
	//判断内容，获取区块详情，然后发送
	if payload.Type=="block"{
		fmt.Printf("payload.ID:%x\n",payload.ID)
		block ,err:= bc.GetBlock([]byte(payload.ID))
		if err!=nil{
			log.Panic(err)
		}
		fmt.Println("g6: ",payload.AddrFrom)
		sendBlock(payload.AddrFrom,&block)
	}
}

// 区块信息发送内容，地址，区块切片
type blocksend struct {
	AddrFrom string
	Block []byte
}

//发送区块信息
func sendBlock(addr string, block *Block) {
	fmt.Println("发送block: ",addr)
	data:= blocksend{nodeAddress,block.Serialize()}
	payload := gobEncode(data)
	request:= append(commandToBytes("block"),payload...)
	sendData(addr,request)
}

//Inv 的数据处理
func handleInv(request []byte, bc *Blockchain) {
	var buff bytes.Buffer
	var payload inv
	buff.Write(request[commonLength:])
	dec:= gob.NewDecoder(&buff)
	err:= dec.Decode(&payload)
	if err !=nil{
		log.Panic(err)
	}
	fmt.Printf("Recieve inventory %d,%s",len(payload.Items),payload.Type)
	if payload.Type =="block"{
		//区块信息
		blockInTransit = payload.Items
		//获取最近一个区块
		blockHash:= payload.Items[0]
		//
		sendGetData(payload.AddrFrom,"block",blockHash)
		newInTransit := [][]byte{}
		for _,b:= range blockInTransit{
			//判断自身区块与外部区块的最新一个区块是否相同，不同则加入至newInTransit
			if bytes.Compare(b,blockHash)!=0{
				newInTransit = append(newInTransit,b)
			}
		}
		blockInTransit =  newInTransit
	}
}

//获取区块数据结构体
type getdata struct {
	AddrFrom string
	Type string
	ID []byte
}

//发送获取区块详情
func sendGetData(addr string, kind string, id []byte) {
	payload:= gobEncode(getdata{nodeAddress,kind,id})
	request:= append(commandToBytes("getdata"),payload...)
	sendData(addr,request)
}


func handleGetBlock(request []byte, bc *Blockchain) {

	var buff bytes.Buffer
	var payload getblocks

	buff.Write(request[commonLength:])
	//获取命令传递的内容
	dec:= gob.NewDecoder(&buff)
	err:= dec.Decode(&payload)
	//解码
	if err !=nil{
		log.Panic(err)
	}
	//获取自身区块哈希信息
	block:= bc.Getblockhash()
	fmt.Println("sendenv: ",payload.Addrfrom)
	//接收地址，自身区块情况
	sendInv(payload.Addrfrom,"block",block)
}


//数据结构体 - 地址-类型-区块信息哈希
type inv struct {
	AddrFrom string
	Type string
	Items [][]byte
}



func sendInv(addr string, kind string, items [][]byte) {
	//内容结构
	inventory:= inv{nodeAddress,kind,items}
	//序列化
	payload := gobEncode(inventory)
	//请求体构建
	request := append(commandToBytes("inv"),payload...)
	//发送
	sendData(addr,request)
}

//版本信息处理
func handleVersion(request []byte, bc *Blockchain) {
	var buff bytes.Buffer
	var payload Version
	//获取命令后的内容
	buff.Write(request[commonLength:])

	dec:=gob.NewDecoder(&buff)
	//反序列化
	err:= dec.Decode(&payload)

	if err!=nil{
		log.Panic(err)
	}
	payload.String()
	//获取自身高度
	myBestHeight := bc.GetBestHeight()
	//外部方高度
	foreignerBestHeight :=  payload.BestHeight

	if myBestHeight < foreignerBestHeight{
		//如果小于，获取外部节点信息
		sendGetBlock(payload.AddrFrom)
	}else{
		//如果大于，把自身区块版本发送出去
		sendVersion(payload.AddrFrom,bc)
	}
	//如果已知列表中是否有当前地址，没有则添加进入
	if !nodeIsKnow(payload.AddrFrom){
		knownNodes = append(knownNodes,payload.AddrFrom)
	}

}

// 存储请求地址的结构体
type getblocks struct {
	Addrfrom string
}

//获取外部区块信息
func sendGetBlock(address string) {
	payload:=  gobEncode(getblocks{nodeAddress})
	//请求内容构建
	request:= append(commandToBytes("getblocks"),payload...)
	//请求发送
	sendData(address,request)
}

//判断节点是否存在已知列表
func nodeIsKnow(addr string) bool {
	for _,node :=range knownNodes{
		if node ==addr{
			return true
		}
	}
	return false
}


//发送版本号，把自己的版本号发送给地址
func sendVersion(addr string, bc *Blockchain) {
	//获取高度
	bestHeight :=bc.GetBestHeight()
	//构建信息，并且序列化
	payload := gobEncode(Version{nodeversion,bestHeight,nodeAddress})
	//request内容
	request:=append(commandToBytes("version"),payload...)
	//发送
	sendData(addr,request)
}

// 数据发送函数
func sendData(addr string, data []byte) {
	con,err := net.Dial("tcp",addr)  //实例协议和地址
	if err !=nil{
		//如果报错，那么可能是节点不可用
		fmt.Printf("%s is no available",addr)

		var updateNodes []string
		//遍历已知节点，更新发送地址
		for _,node:=range knownNodes{
			if node !=addr{
				//如果当前节点不是当前使用的，就更新
				updateNodes = append(updateNodes,node)
			}
		}

		knownNodes = updateNodes
	}
	defer con.Close()
	//把数据复制进con通道，请求就完成了发送
	_,err = io.Copy(con,bytes.NewReader(data))

	if err !=nil{
		log.Panic(err)
	}
}

// 命令转成指定长度
func commandToBytes(command string) []byte {
	var bytes [commonLength]byte
	//新建数组，遍历传入
	for i,c:= range command{
		bytes[i] = byte(c)
	}
	return bytes[:]
}

//命令还原
func bytesToCommand(bytes []byte) string{
	var command []byte
	//创建动态字节数组，遍历获取
	for _,b:=range bytes{
		if b!=0x00{
			command = append(command,b)
		}
	}
	//数组转成str
	return fmt.Sprintf("%s",command)
}

// 通用序列化接口
func gobEncode(data interface{}) []byte{
	var buff bytes.Buffer
	//传入缓存区指针
	enc := gob.NewEncoder(&buff)
	//传入内容
	err := enc.Encode(data)

	if err!=nil{
		log.Panic(err)

	}
	//输出序列化内容
	return buff.Bytes()

}