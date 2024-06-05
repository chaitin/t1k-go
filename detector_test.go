package t1k

import (
	"os"
	"reflect"
	"testing"
)

const (
	NormalReq       = "GET / HTTP/1.1\r\nHost: example.com\r\n\r\n"
	SqlInjectionReq = "GET /?id=1%20AND%201=1 HTTP/1.1\r\nHost: example.com\r\n\r\n"
	Res             = "HTTP/1.1 200 OK\r\nContent-Length: 11\r\nContent-Type: text/html\r\n\r\nhello,world"
)

func TestDetector_DetectorRequest(t *testing.T) {
	d := NewDetector(Config{Addr: os.Getenv("DETECTOR_ADDR")})
	res, err := d.DetectorRequestStr(NormalReq)
	if err != nil {
		t.Fatal(err)
	}
	if res.Allowed() {
		t.Log("req allowed as expected")
	} else {
		t.Fatalf("req blocked but expected to be allowed")
	}
}

func TestDetector_DetectorRequest_SqlInjection(t *testing.T) {
	d := NewDetector(Config{Addr: os.Getenv("DETECTOR_ADDR")})
	res, err := d.DetectorRequestStr(SqlInjectionReq)
	if err != nil {
		t.Fatal(err)
	}
	if res.Allowed() {
		t.Fatalf("req allowed but expected to be blocked")
	} else {
		t.Logf("req blocked %d by event id %s as expected", res.StatusCode(), res.EventID())
	}
}

func TestDetector_DetectorResponseStr(t *testing.T) {
	d := NewDetector(Config{Addr: os.Getenv("DETECTOR_ADDR")})
	res, err := d.DetectorResponseStr(NormalReq, Res)
	if err != nil {
		t.Fatal(err)
	}
	if res.Allowed() {
		t.Log("req allowed as expected")
	} else {
		t.Fatalf("req blocked but expected to be allowed")
	}
}

func Test_reverseStrSlice(t *testing.T) {
	type args struct {
		arr []string
	}
	tests := []struct {
		name string
		args args
		want []string
	}{
		{
			name: "case1",
			args: args{
				arr: []string{"a", "b", "c"},
			},
			want: []string{"c", "b", "a"},
		},
		{
			name: "case2",
			args: args{
				arr: []string{"a"},
			},
			want: []string{"a"},
		},
		{
			name: "case3",
			args: args{
				arr: []string{},
			},
			want: []string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := reverseStrSlice(tt.args.arr); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("reverseStrSlice() = %v, want %v", got, tt.want)
			}
		})
	}
}
