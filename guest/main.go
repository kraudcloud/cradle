package main

/*
import (
	"bufio"
	"github.com/kraudcloud/cradle/proto"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"github.com/creack/pty"
	"github.com/mdlayher/vsock"
	"github.com/oraoto/go-pidfd"
	"golang.org/x/sys/unix"
	"io"
	"net"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
)
*/



import (
	"os"
	"time"
)


func main() {

	uinit();

	lo, err := os.Create("/dev/kmsg")
	if err == nil {
		log.Out = lo
	}
	log.Println("\033[1;34m ---- KRAUDCLOUD CRADLE ---- \033[0m")

	wdinit();

	makedev();
	mountnvme();
	unpack();

	axyinit();
	shell();
	for ;; { time.Sleep(time.Minute) }
}
