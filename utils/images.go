package utils

import (
	"image"
	"image/draw"
)

// CombineTwoImages combines two images with img1 being on the left and img2 on the right. returns the resulting image
func CombineTwoImages(img1 image.Image, img2 image.Image) image.Image {

	//starting position of the second image (bottom left)
	sp2 := image.Point{img1.Bounds().Dx(), 0}

	//new rectangle for the second image
	r2 := image.Rectangle{sp2, sp2.Add(img2.Bounds().Size())}

	//rectangle for the big image
	r := image.Rectangle{image.Point{0, 0}, r2.Max}
	rgba := image.NewRGBA(r)

	draw.Draw(rgba, img1.Bounds(), img1, image.Point{0, 0}, draw.Src)
	draw.Draw(rgba, r2, img2, image.Point{0, 0}, draw.Src)

	return rgba.SubImage(r)
}
