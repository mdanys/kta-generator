package main

import (
	"fmt"
	"log"
)

func main() {
	// Example 1: blank card (name area empty, ready for handwriting)
	blank := KTAData{
		NomorKTA:   "8831740500000001",
		Kecamatan:  "Kecamatan Kelapa Gading",
		Kota:       "KOTA JAKARTA UTARA",
		Provinsi:   "DKI Jakarta",
		LogoPath:   "assets/logo.png",
		OutputPath: "output_blank.png",
	}

	if err := GenerateKTACard(blank); err != nil {
		log.Fatalf("failed to generate blank card: %v", err)
	}
	fmt.Println("✓ Generated:", blank.OutputPath)

	// Example 2: filled card (name already written)
	filled := KTAData{
		NomorKTA:    "8831740500000002",
		NamaAnggota: "Siti Rahmawati",
		Kecamatan:   "Kecamatan Kelapa Gading",
		Kota:        "KOTA JAKARTA UTARA",
		Provinsi:    "DKI Jakarta",
		LogoPath:    "assets/logo.png",
		OutputPath:  "output_filled.png",
	}

	if err := GenerateKTACard(filled); err != nil {
		log.Fatalf("failed to generate filled card: %v", err)
	}
	fmt.Println("✓ Generated:", filled.OutputPath)
}
