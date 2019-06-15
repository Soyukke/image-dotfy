package main

import (
	"log"
    "fmt"
	"net/http"
	"html/template"
    "os"
	"strconv"
	"strings"
	"image"
	"image/png"
	"image/color"
	"github.com/nfnt/resize"
	"bytes"
	"bufio"
	"encoding/base64"
	"math"
	"math/rand"
	"time"
)

// main page
func handler(w http.ResponseWriter, r *http.Request) {
	tmpl := template.Must(template.ParseFiles("index.html"))
	if err := tmpl.ExecuteTemplate(w, "index.html", nil); err != nil {
		log.Fatal(err)
	}
}

// return edited image
func editimage(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// get parameters
	height_str := r.FormValue("height")
	width_str := r.FormValue("width")
	nbits_str := r.FormValue("nbits")
	file, _, err := r.FormFile("image")
	if err != nil {
		fmt.Println(err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	defer file.Close()

	img, _, err := image.Decode(file)
	if err != nil {
		fmt.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// string -> int
	height, _ := strconv.Atoi(height_str)
	width, _ := strconv.Atoi(width_str)
	nbits, _ := strconv.Atoi(strings.Split(nbits_str, " ")[0])

	fmt.Println(height, width, nbits)

	// newImg, ok := img.(*image.RGBA)
	var imgStr string
	newImg := resizeAndMakeImage(img, uint(width), uint(height), int(math.Pow(2, float64(nbits))))
	// nilが返ってきたらreturn
	if newImg == nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	imgStr = encodeBase64(newImg)

	tmpl := template.Must(template.ParseFiles("image.html"))
	// 画像base64
	if err := tmpl.ExecuteTemplate(w, "image.html", imgStr); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Fatal(err)
		return
	}

}

func resizeAndMakeImage(img image.Image, width uint, height uint, n_cluster int) *image.RGBA {
	// img, ok := img.(*image.RGBA)
	fmt.Println("received image")
	img_resized := resize.Resize(width, height, img, resize.Lanczos3)
	fmt.Println("resized image")
	if x, ok := img_resized.(*image.RGBA); ok {
		fmt.Println("convert to *image.RGBA")
		kmeans(x, n_cluster)
		return x
	}
	return nil
}

func ImageToUnit8s(img *image.RGBA) []color.RGBA {
	n_pixels := (img.Rect.Max.X - img.Rect.Min.X) * (img.Rect.Max.Y - img.Rect.Min.Y)
	vcolor := make([]color.RGBA, n_pixels)
	index := 0
	for x := img.Rect.Min.X; x < img.Rect.Max.X; x++ {
		for y := img.Rect.Min.Y; y < img.Rect.Max.Y; y++ {
			vcolor[index], _ = img.At(x, y).(color.RGBA)
			index += 1
		}
	}
	return vcolor
}

func UpdataImageByUint8s(img *image.RGBA, vcolor []color.RGBA) {
	index := 0
	for x := img.Rect.Min.X; x < img.Rect.Max.X; x++ {
		for y := img.Rect.Min.Y; y < img.Rect.Max.Y; y++ {
			img.Set(x, y, vcolor[index])
			index += 1
		}
	}
}

func kmeans(img *image.RGBA, n_cluster int) {
	// colorの距離を計算する関数
	color_distance := func(color1 color.RGBA, color2 color.RGBA) float64 {
		r1, g1, b1, a1 := color1.R, color1.G, color1.B, color1.A
		r2, g2, b2, a2 := color2.R, color2.G, color2.B, color2.A
		r1f := float64(r1)
		g1f := float64(g1)
		b1f := float64(b1)
		a1f := float64(a1)

		r2f := float64(r2)
		g2f := float64(g2)
		b2f := float64(b2)
		a2f := float64(a2)

		distance := math.Sqrt(math.Pow(r2f-r1f, 2) + math.Pow(g2f-g1f, 2) + math.Pow(b2f-b1f, 2) + math.Pow(a2f-a1f, 2))
		return distance
	}

	vcolor := ImageToUnit8s(img)
	// ベクトル
	n_pixels := len(vcolor)
	// クラスタ中心数 = 8 * ビット数
	// クラスタ中心のベクトル
	vcluster := make([]color.RGBA, n_cluster)
	fmt.Println(n_pixels)
	// 残差
	residual := float32(n_pixels)

	// 初期化
	rand.Seed(time.Now().UnixNano())
	vtype := make([]int, n_pixels)
	for i := 0; i< len(vtype); i++ {
		vtype[i] = rand.Intn(n_cluster)
	}

	// k-means
	n_iter := 0
	for residual > 0 && n_iter < 30 {
		residual = 0
		// vclusterの更新
		// vtype から filterして特定のcluter中心に対応するcolorを取り出して平均を計算する
		for i:=0;i<n_cluster;i++ {
			// vtypeのうち，cluster i に属するindexのみを取り出す
			vtype_cluster_i := make([]int, 0)
			for index, type_cluster := range vtype {
				if type_cluster == i {
					vtype_cluster_i = append(vtype_cluster_i, index)
				}
			}
			// vtype_cluster_iが0個ならスルー
			if len(vtype_cluster_i) == 0 {
				continue
			}
			n_vtype_cluster_i := float64(len(vtype_cluster_i))
			// type_cluster_i
			r_sum, g_sum, b_sum, a_sum := 0.0, 0.0, 0.0, 0.0
			for _, type_cluster_i := range vtype_cluster_i {
				color_ := vcolor[type_cluster_i]
				r_sum += float64(color_.R)/n_vtype_cluster_i
				g_sum += float64(color_.G)/n_vtype_cluster_i
				b_sum += float64(color_.B)/n_vtype_cluster_i
				a_sum += float64(color_.A)/n_vtype_cluster_i
			}
			// クラスタ中心の色更新
			vcluster[i] = color.RGBA{uint8(r_sum), uint8(g_sum), uint8(b_sum), uint8(a_sum)}
		}
	
		// vtypeの更新
		for vtype_index, color_ := range vcolor {
			// どのclusterに距離が近いか
			cluster_index_min := vtype[vtype_index]
			distance_min := 1000.0
			for cluster_index, cluster := range vcluster {
				distance := color_distance(color_, cluster)
				if distance < distance_min {
					distance_min = distance
					cluster_index_min = cluster_index
				}
			}
			if cluster_index_min != vtype[vtype_index] {
				residual += 1
			}
			vtype[vtype_index] = cluster_index_min
		}

		n_iter += 1
		fmt.Println("iter residual = ", n_iter, residual)
	}

	// 色をcluster中心の色に書き換える
	for index:=0;index<n_pixels;index++ {
		vcolor[index] = vcluster[vtype[index]]
	}
	UpdataImageByUint8s(img, vcolor)
}

func encodeBase64(img *image.RGBA) string {
	buf := new(bytes.Buffer)
	bio := bufio.NewWriter(buf)
	err := png.Encode(bio, img)
	if err != nil {
		log.Fatal(err)
	}
	bio.Flush()
	return base64.StdEncoding.EncodeToString(buf.Bytes())
}

func main() {
	port, _ := strconv.Atoi(os.Getenv("PORT"))
	fmt.Printf("Starting server at Port %d", port)
	// localhost:port
	http.HandleFunc("/", handler)
	// localhost:port/editimage
	http.HandleFunc("/editimage", editimage)
	// cssファイルを読み込めるようにする
	http.Handle("/css/", http.StripPrefix("/css/", http.FileServer(http.Dir("css"))))
	// 画像を一時的においておく
	http.Handle("/tmp/", http.StripPrefix("/tmp/", http.FileServer(http.Dir("tmp"))))
	// サーバ起動
    http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
}