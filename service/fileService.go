package service

import (
	"archive/tar"
	"compress/gzip"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"time"

	"image"
	"image/color"
	"image/draw"
	"image/jpeg"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/goregular"

	"github.com/akhilrex/podgrab/db"
	"github.com/akhilrex/podgrab/internal/sanitize"
	"github.com/disintegration/imaging"
	stringy "github.com/gobeam/stringy"
	"github.com/golang/freetype"
	"github.com/golang/freetype/truetype"
)

func Download(link string, episodeTitle string, podcastName string, prefix string) (string, error) {
	if link == "" {
		return "", errors.New("Download path empty")
	}
	client := httpClient()

	req, err := getRequest(link)
	if err != nil {
		Logger.Errorw("Error creating request: "+link, err)
	}

	resp, err := client.Do(req)
	if err != nil {
		Logger.Errorw("Error getting response: "+link, err)
		return "", err
	}

	fileName := getFileName(link, episodeTitle, ".mp3")
	if prefix != "" {
		fileName = fmt.Sprintf("%s-%s", prefix, fileName)
	}
	folder := createDataFolderIfNotExists(podcastName)
	finalPath := path.Join(folder, fileName)

	if _, err := os.Stat(finalPath); !os.IsNotExist(err) {
		changeOwnership(finalPath)
		return finalPath, nil
	}

	file, err := os.Create(finalPath)
	if err != nil {
		Logger.Errorw("Error creating file"+link, err)
		return "", err
	}
	defer resp.Body.Close()
	_, erra := io.Copy(file, resp.Body)
	//fmt.Println(size)
	defer file.Close()
	if erra != nil {
		Logger.Errorw("Error saving file"+link, err)
		return "", erra
	}
	changeOwnership(finalPath)
	return finalPath, nil

}

func GetPodcastLocalImagePath(link string, podcastName string) string {
	fileName := getFileName(link, "folder", ".jpg")
	folder := createDataFolderIfNotExists(podcastName)

	finalPath := path.Join(folder, fileName)
	return finalPath
}

func CreateNfoFile(podcast *db.Podcast) error {
	fileName := "album.nfo"
	folder := createDataFolderIfNotExists(podcast.Title)

	finalPath := path.Join(folder, fileName)

	type NFO struct {
		XMLName xml.Name `xml:"album"`
		Title   string   `xml:"title"`
		Type    string   `xml:"type"`
		Thumb   string   `xml:"thumb"`
	}

	toSave := NFO{
		Title: podcast.Title,
		Type:  "Broadcast",
		Thumb: podcast.Image,
	}
	out, err := xml.MarshalIndent(toSave, " ", "  ")
	if err != nil {
		return err
	}
	toPersist := xml.Header + string(out)
	return ioutil.WriteFile(finalPath, []byte(toPersist), 0644)
}

func DownloadPodcastCoverImage(link string, podcastName string) (string, error) {
	if link == "" {
		return "", errors.New("Download path empty")
	}
	client := httpClient()
	req, err := getRequest(link)
	if err != nil {
		Logger.Errorw("Error creating request: "+link, err)
		return "", err
	}

	resp, err := client.Do(req)
	if err != nil {
		Logger.Errorw("Error getting response: "+link, err)
		return "", err
	}

	fileName := getFileName(link, "folder", ".jpg")
	folder := createDataFolderIfNotExists(podcastName)

	finalPath := path.Join(folder, fileName)
	if _, err := os.Stat(finalPath); !os.IsNotExist(err) {
		changeOwnership(finalPath)
		return finalPath, nil
	}

	file, err := os.Create(finalPath)
	if err != nil {
		Logger.Errorw("Error creating file"+link, err)
		return "", err
	}
	defer resp.Body.Close()
	_, erra := io.Copy(file, resp.Body)
	//fmt.Println(size)
	fmt.Println("Cover Image has been downloaded to :" + finalPath)
	defer file.Close()
	if erra != nil {
		Logger.Errorw("Error saving file"+link, err)
		return "", erra
	}
	changeOwnership(finalPath)
	addLabelToImage(finalPath)
	return finalPath, nil
}

func DownloadImage(link string, episodeId string, podcastName string) (string, error) {
	if link == "" {
		return "", errors.New("Download path empty")
	}
	client := httpClient()
	req, err := getRequest(link)
	if err != nil {
		Logger.Errorw("Error creating request: "+link, err)
		return "", err
	}

	resp, err := client.Do(req)
	if err != nil {
		Logger.Errorw("Error getting response: "+link, err)
		return "", err
	}

	fileName := getFileName(link, episodeId, ".jpg")
	folder := createDataFolderIfNotExists(podcastName)
	imageFolder := createFolder("images", folder)
	finalPath := path.Join(imageFolder, fileName)

	if _, err := os.Stat(finalPath); !os.IsNotExist(err) {
		changeOwnership(finalPath)
		return finalPath, nil
	}

	file, err := os.Create(finalPath)
	if err != nil {
		Logger.Errorw("Error creating file"+link, err)
		return "", err
	}
	defer resp.Body.Close()
	_, erra := io.Copy(file, resp.Body)
	//fmt.Println(size)
	defer file.Close()
	fmt.Println("Image has been downloaded to :" + finalPath)
	addLabelToImage(finalPath)
	if erra != nil {
		Logger.Errorw("Error saving file"+link, err)
		return "", erra
	}
	changeOwnership(finalPath)
	return finalPath, nil
}

func addLabelToImage(path string) {
	fmt.Println("Adding Label to the file at : " + path)
	// Open the image file.
	file, err := os.Open(path)
	if err != nil {
		Logger.Errorw("Error opening file "+path, err)
	}
	defer file.Close()

	// Decode the image.
	img, _, err := image.Decode(file)
	if err != nil {
		Logger.Errorw("Error decoding image file", err)
	}

	// Create a new image with the same dimensions as the original image.
	dst := imaging.New(img.Bounds().Dx(), img.Bounds().Dy(), color.NRGBA{0, 0, 0, 0})

	// Overlay the original image onto the new image.
	draw.Draw(dst, dst.Bounds(), img, image.Point{0, 0}, draw.Src)

	// Add the text to the new image.
	text := "PodGrabbed"
	fontz, _ := truetype.Parse(goregular.TTF)

	// Create a new font face with the desired size.
	fontSize := float64(img.Bounds().Max.Y / 10)
	fontFace := truetype.NewFace(fontz, &truetype.Options{Size: fontSize})

	pt := freetype.Pt(int(fontSize), img.Bounds().Max.Y-int(fontSize))
	drawer := &font.Drawer{
		Dst:  dst,
		Src:  image.White,
		Face: fontFace,
		Dot:  pt,
	}

	// Add a highlight behind the text.
	highlightColor := color.RGBA{0, 0, 0, 255}
	textWidth := drawer.MeasureString(text).Ceil()
	textHeight := int(fontSize)
	marginGap := int(fontSize)
	borderGap := int(fontSize) / 20

	topLeftX := marginGap - borderGap
	topLeftY := img.Bounds().Max.Y - (marginGap + textHeight)
	botRightX := topLeftX + textWidth + borderGap
	botRightY := topLeftY + textHeight + borderGap*4
	highlightRect := image.Rect(topLeftX, topLeftY, botRightX, botRightY)
	draw.Draw(dst, highlightRect, &image.Uniform{highlightColor}, image.Point{}, draw.Src)
	drawer.DrawString(text)

	outPath := path + ".tmp"
	outFile, err := os.Create(outPath)
	if err != nil {
		log.Fatal(err)
	}
	defer outFile.Close()

	err = jpeg.Encode(outFile, dst, nil)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Watermaked Image created at :" + outPath)
	fmt.Println("Will now be overwriting input Image.")
	err = os.Rename(outPath, path)
	if err != nil {
		fmt.Println(err)
	}
}

func changeOwnership(path string) {
	uid, err1 := strconv.Atoi(os.Getenv("PUID"))
	gid, err2 := strconv.Atoi(os.Getenv("PGID"))
	fmt.Println(path)
	if err1 == nil && err2 == nil {
		fmt.Println(path + " : Attempting change")
		os.Chown(path, uid, gid)
	}

}
func DeleteFile(filePath string) error {
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return err
	}
	if err := os.Remove(filePath); err != nil {
		return err
	}
	return nil
}
func FileExists(filePath string) bool {
	_, err := os.Stat(filePath)
	return err == nil

}

func GetAllBackupFiles() ([]string, error) {
	var files []string
	folder := createConfigFolderIfNotExists("backups")
	err := filepath.Walk(folder, func(path string, info os.FileInfo, err error) error {
		if !info.IsDir() {
			files = append(files, path)
		}
		return nil
	})
	sort.Sort(sort.Reverse(sort.StringSlice(files)))
	return files, err
}

func GetFileSize(path string) (int64, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

func deleteOldBackup() {
	files, err := GetAllBackupFiles()
	if err != nil {
		return
	}
	if len(files) <= 5 {
		return
	}

	toDelete := files[5:]
	for _, file := range toDelete {
		fmt.Println(file)
		DeleteFile(file)
	}
}

func GetFileSizeFromUrl(url string) (int64, error) {
	resp, err := http.Head(url)
	if err != nil {
		return 0, err
	}

	// Is our request ok?

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("Did not receive 200")
	}

	size, err := strconv.Atoi(resp.Header.Get("Content-Length"))
	if err != nil {
		return 0, err
	}

	return int64(size), nil
}

func CreateBackup() (string, error) {

	backupFileName := "podgrab_backup_" + time.Now().Format("2006.01.02_150405") + ".tar.gz"
	folder := createConfigFolderIfNotExists("backups")
	configPath := os.Getenv("CONFIG")
	tarballFilePath := path.Join(folder, backupFileName)
	file, err := os.Create(tarballFilePath)
	if err != nil {
		return "", errors.New(fmt.Sprintf("Could not create tarball file '%s', got error '%s'", tarballFilePath, err.Error()))
	}
	defer file.Close()

	dbPath := path.Join(configPath, "podgrab.db")
	_, err = os.Stat(dbPath)
	if err != nil {
		return "", errors.New(fmt.Sprintf("Could not find db file '%s', got error '%s'", dbPath, err.Error()))
	}
	gzipWriter := gzip.NewWriter(file)
	defer gzipWriter.Close()

	tarWriter := tar.NewWriter(gzipWriter)
	defer tarWriter.Close()

	err = addFileToTarWriter(dbPath, tarWriter)
	if err == nil {
		deleteOldBackup()
	}
	return backupFileName, err
}

func addFileToTarWriter(filePath string, tarWriter *tar.Writer) error {
	file, err := os.Open(filePath)
	if err != nil {
		return errors.New(fmt.Sprintf("Could not open file '%s', got error '%s'", filePath, err.Error()))
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return errors.New(fmt.Sprintf("Could not get stat for file '%s', got error '%s'", filePath, err.Error()))
	}

	header := &tar.Header{
		Name:    filePath,
		Size:    stat.Size(),
		Mode:    int64(stat.Mode()),
		ModTime: stat.ModTime(),
	}

	err = tarWriter.WriteHeader(header)
	if err != nil {
		return errors.New(fmt.Sprintf("Could not write header for file '%s', got error '%s'", filePath, err.Error()))
	}

	_, err = io.Copy(tarWriter, file)
	if err != nil {
		return errors.New(fmt.Sprintf("Could not copy the file '%s' data to the tarball, got error '%s'", filePath, err.Error()))
	}

	return nil
}
func httpClient() *http.Client {
	client := http.Client{
		CheckRedirect: func(r *http.Request, via []*http.Request) error {
			//	r.URL.Opaque = r.URL.Path
			return nil
		},
	}

	return &client
}

func getRequest(url string) (*http.Request, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	setting := db.GetOrCreateSetting()
	if len(setting.UserAgent) > 0 {
		req.Header.Add("User-Agent", setting.UserAgent)
	}

	return req, nil
}

func createFolder(folder string, parent string) string {
	folder = cleanFileName(folder)
	//str := stringy.New(folder)
	folderPath := path.Join(parent, folder)
	if _, err := os.Stat(folderPath); os.IsNotExist(err) {
		os.MkdirAll(folderPath, 0777)
		changeOwnership(folderPath)
	}
	return folderPath
}

func createDataFolderIfNotExists(folder string) string {
	dataPath := os.Getenv("DATA")
	return createFolder(folder, dataPath)
}
func createConfigFolderIfNotExists(folder string) string {
	dataPath := os.Getenv("CONFIG")
	return createFolder(folder, dataPath)
}

func deletePodcastFolder(folder string) error {
	return os.RemoveAll(createDataFolderIfNotExists(folder))
}

func getFileName(link string, title string, defaultExtension string) string {
	fileUrl, err := url.Parse(link)
	checkError(err)

	parsed := fileUrl.Path
	ext := filepath.Ext(parsed)

	if len(ext) == 0 {
		ext = defaultExtension
	}
	//str := stringy.New(title)
	str := stringy.New(cleanFileName(title))
	return str.KebabCase().Get() + ext

}

func cleanFileName(original string) string {
	return sanitize.Name(original)
}

func checkError(err error) {
	if err != nil {
		panic(err)
	}
}
