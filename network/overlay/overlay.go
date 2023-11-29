package overlay

type Overlay struct {
	NodeName string
	vxlan100 string
}

// f6:35:84:38:60:f1
// ip neighbor add 172.18.20.2 lladdr 16:8f:3f:90:b9:2e dev vxlan100
// bridge fdb append 16:8f:3f:90:b9:2e dev vxlan100 dst 192.168.245.172

// sudo ip link add vxlan100 type vxlan \
//     id 100 \
//     local 192.168.245.168 \
//     dev ens33 \
//     dstport 4789 \
//     nolearning

// 16:8f:3f:90:b9:2e
// ip neighbor add 172.18.10.2 lladdr f6:35:84:38:60:f1 dev vxlan100
// bridge fdb append f6:35:84:38:60:f1 dev vxlan100 dst 192.168.245.168

// 	sudo ip link add vxlan100 type vxlan \
//     id 100 \
//     local 192.168.245.172 \
//     dev ens33 \
//     dstport 4789 \
//     nolearning
