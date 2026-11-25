// 本程序只用于验收Node→Go真实Protobuf字节，不加入业务服务运行路径。
package main

import (
	pb "collaboration/contracts/internalv1"
	"encoding/json"
	"google.golang.org/protobuf/proto"
	"io"
	"os"
	"strconv"
)

func main() {
	data, err := io.ReadAll(io.LimitReader(os.Stdin, 1<<20))
	var value pb.IdentityAdmission
	if err != nil || proto.Unmarshal(data, &value) != nil {
		os.Exit(1)
	}
	_ = json.NewEncoder(os.Stdout).Encode(map[string]string{"actor_id": strconv.FormatUint(value.ActorId, 10), "epoch": strconv.FormatUint(value.Sources[0].Epoch, 10)})
}
