package client

import (
	"context"
	"github.com/fejnartal/bosh-ghrelcli/config"
	"github.com/google/go-github/v47/github"
	"golang.org/x/oauth2"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
)

type GHRelBlobstore struct {
	ctx            context.Context
	githubClient   *github.Client
	ghrelcliConfig *config.GHRelCli
}

func (client *GHRelBlobstore) validateRemoteConfig() error {
	return nil
}

func New(ctx context.Context, ghrelcliConfig *config.GHRelCli) (*GHRelBlobstore, error) {
	var httpC *http.Client

	if len(ghrelcliConfig.AccessToken) == 0 {
		// Unprivileged unauthenticated access
		httpC = &http.Client{}
	} else {
		// Authenticated with Oauth token
		oauthHttpC := oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: ghrelcliConfig.AccessToken},
		)
		httpC = oauth2.NewClient(ctx, oauthHttpC)
	}

	client := github.NewClient(httpC)

	return &GHRelBlobstore{
		ctx:            ctx,
		githubClient:   client,
		ghrelcliConfig: ghrelcliConfig,
	}, nil
}

func (client *GHRelBlobstore) Get(src string, dest io.WriterAt) error {
	owner := strings.Split(client.ghrelcliConfig.Repository, "/")[0]
	repo := strings.Split(client.ghrelcliConfig.Repository, "/")[1]

	release, _, err := client.githubClient.Repositories.GetReleaseByTag(client.ctx, owner, repo, client.ghrelcliConfig.TagName)
	if err != nil {
		return err
	}

	var desiredAssetID *int64
	curPage := 1
	for desiredAssetID == nil {
		assets, resp, err := client.githubClient.Repositories.ListReleaseAssets(client.ctx, owner, repo, *release.ID, &github.ListOptions{ Page: curPage })
		if err != nil {
			return err
		}
		for _, asset := range assets {
			if *asset.Name == src {
				desiredAssetID = asset.ID
				break
			}
		}
		if curPage == resp.LastPage || curPage > resp.NextPage {
			// According to github-go docs NextPage may be set to the zero value for
			// responses that are not part of a paginated set or if there are no additional pages.
			break
		}
		curPage = resp.NextPage
	}

	assetReader, _, err := client.githubClient.Repositories.DownloadReleaseAsset(client.ctx, owner, repo, *desiredAssetID, http.DefaultClient)
	if err != nil {
		return err
	}
	defer assetReader.Close()

	var alreadyCopied int64
	alreadyCopied = 0
	for {
		b := make([]byte, 8)
		n, err := assetReader.Read(b)
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		n, err = dest.WriteAt(b[0:n], alreadyCopied)
		if err != nil {
			return err
		}
		alreadyCopied += int64(n)
	}
	return nil
}

func (client *GHRelBlobstore) Put(src io.ReadSeeker, dest string) error {
	owner := strings.Split(client.ghrelcliConfig.Repository, "/")[0]
	repo := strings.Split(client.ghrelcliConfig.Repository, "/")[1]

	release, response, err := client.githubClient.Repositories.GetReleaseByTag(client.ctx, owner, repo, client.ghrelcliConfig.TagName)
	if err != nil {
		if response == nil || response.StatusCode != http.StatusNotFound {
			return err
		}

		newRelease := &github.RepositoryRelease{
			TagName: &client.ghrelcliConfig.TagName,
			Name:    &client.ghrelcliConfig.TagName,
		}
		release, response, err = client.githubClient.Repositories.CreateRelease(client.ctx, owner, repo, newRelease)
		if err != nil {
			return err
		}
	}

	tmpfile, err := ioutil.TempFile("", dest)
	if err != nil {
		return err
	}
	defer os.Remove(tmpfile.Name())
	_, err = io.Copy(tmpfile, src)
	if err != nil {
		return err
	}
	assetFile, err := os.Open(tmpfile.Name())
	tmpfile.Close()

	_, response, err = client.githubClient.Repositories.UploadReleaseAsset(client.ctx, owner, repo, *release.ID, &github.UploadOptions{Name: dest}, assetFile)
	return err
}
