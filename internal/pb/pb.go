package pb //provable broadcast

import (
	"Buada_BFT/internal/party"
	"Buada_BFT/pkg/core"
	"Buada_BFT/pkg/protobuf"
	"bytes"
	"context"
	"log"

	"go.dedis.ch/kyber/v3/pairing"
	"go.dedis.ch/kyber/v3/sign/tbls"
	"golang.org/x/crypto/sha3"
)

//Sender is run by the sender of a instance of provable broadcast
func Sender(ctx context.Context, p *party.HonestParty, ID []byte, value []byte, validation []byte) ([]byte, bool) {
	valueMessage := core.Encapsulation("Value", ID, p.PID, &protobuf.Value{
		Value:      value,
		Validation: validation,
	})

	p.Broadcast(valueMessage)

	sigs := [][]byte{}
	h := sha3.Sum512(value)
	var buf bytes.Buffer
	buf.Write([]byte("Echo"))
	buf.Write(ID)
	buf.Write(h[:])
	sm := buf.Bytes()

	for i := uint32(0); ; i = (i + 1) % p.N {
		select {
		case <-ctx.Done():
			return nil, false
		default:
			m, ok := p.GetMessage(i, "Echo", ID)
			if !ok {
				continue
			}
			payload := core.Decapsulation("Echo", m).(*protobuf.Echo)
			err := tbls.Verify(pairing.NewSuiteBn256(), p.SigPK, sm, payload.Sigshare) //verifyshare("Echo"||ID||h)

			if err == nil {
				sigs = append(sigs, payload.Sigshare)
				if len(sigs) > int(2*p.F) {
					signature, _ := tbls.Recover(pairing.NewSuiteBn256(), p.SigPK, sm, sigs, int(2*p.F+1), int(p.N))
					return signature, true
				}
			}
		}
	}

}

//Receiver is run by the receiver of a instance of provable broadcast
func Receiver(ctx context.Context, p *party.HonestParty, sender uint32, ID []byte, validator func(*party.HonestParty, []byte, uint32, []byte, []byte) error) ([]byte, []byte, bool) {
	var m *protobuf.Message
	var ok bool
	for {
		select {
		case <-ctx.Done():
			return nil, nil, false
		default:
			m, ok = p.GetMessage(sender, "Value", ID)
			if ok {
				payload := (core.Decapsulation("Value", m)).(*protobuf.Value)
				if validator != nil {
					err2 := validator(p, ID, sender, payload.Value, payload.Validation)
					if err2 != nil {
						log.Fatalln(err2)
						return nil, nil, false
					}
				}

				h := sha3.Sum512(payload.Value)
				var buf bytes.Buffer
				buf.Write([]byte("Echo"))
				buf.Write(ID)
				buf.Write(h[:])
				sm := buf.Bytes()
				sigShare, _ := tbls.Sign(pairing.NewSuiteBn256(), p.SigSK, sm) //sign("Echo"||ID||h)

				echoMessage := core.Encapsulation("Echo", ID, p.PID, &protobuf.Echo{
					Sigshare: sigShare,
				})
				p.Send(echoMessage, sender)

				return payload.Value, payload.Validation, true
			}
		}
	}
}
