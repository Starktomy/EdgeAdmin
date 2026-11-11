// edge-sign-release signs a release ZIP sidecar using an offline Ed25519 key.
package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"flag"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/TeaOSLab/EdgeAdmin/internal/utils/upgradesecurity"
)

func main() {
	generate := flag.Bool("generate-key", false, "generate a new offline private key")
	keyFile := flag.String("key", "", "offline Ed25519 PKCS#8 PEM key file")
	keyID := flag.String("key-id", "", "public key identifier configured on the administrator")
	artifact := flag.String("artifact", "", "release ZIP path")
	version := flag.String("version", "", "release version")
	osName := flag.String("os", "linux", "release OS")
	arch := flag.String("arch", "amd64", "release architecture")
	component := flag.String("component", "admin", "release component")
	flag.Parse()
	if *keyFile == "" {
		log.Fatal("-key is required")
	}
	if *generate {
		public, private, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			log.Fatal(err)
		}
		data, err := x509.MarshalPKCS8PrivateKey(private)
		if err != nil {
			log.Fatal(err)
		}
		fp, err := os.OpenFile(*keyFile, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			log.Fatal(err)
		}
		if _, err := fp.Write(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: data})); err != nil {
			fp.Close()
			log.Fatal(err)
		}
		if err := fp.Close(); err != nil {
			log.Fatal(err)
		}
		fmt.Println("Public key:", base64.StdEncoding.EncodeToString(public))
		return
	}
	if *keyID == "" || *artifact == "" || *version == "" {
		log.Fatal("-key-id, -artifact and -version are required")
	}
	keyData, err := os.ReadFile(*keyFile)
	if err != nil {
		log.Fatal(err)
	}
	block, _ := pem.Decode(keyData)
	if block == nil {
		log.Fatal("invalid private key PEM")
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		log.Fatal(err)
	}
	private, ok := parsed.(ed25519.PrivateKey)
	if !ok {
		log.Fatal("an Ed25519 private key is required")
	}
	fp, err := os.Open(*artifact)
	if err != nil {
		log.Fatal(err)
	}
	digest := sha256.New()
	size, err := io.Copy(digest, fp)
	fp.Close()
	if err != nil {
		log.Fatal(err)
	}
	if size <= 0 || size > upgradesecurity.MaxArtifactBytes {
		log.Fatal("artifact exceeds the supported release size")
	}
	manifest := upgradesecurity.Manifest{Component: *component, Version: *version, OS: *osName, Arch: *arch, SHA256: hex.EncodeToString(digest.Sum(nil)), Size: size}
	payload, err := json.Marshal(manifest)
	if err != nil {
		log.Fatal(err)
	}
	envelope := upgradesecurity.SignedManifest{KeyID: *keyID, Payload: base64.StdEncoding.EncodeToString(payload), Signature: base64.StdEncoding.EncodeToString(ed25519.Sign(private, payload))}
	data, err := json.MarshalIndent(envelope, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(*artifact+".manifest.json", append(data, '\n'), 0644); err != nil {
		log.Fatal(err)
	}
}
