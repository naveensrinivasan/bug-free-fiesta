package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gocarina/gocsv"
)

type response struct {
	ID         int    `csv:"ID"`
	Title      string `csv:"Title"`
	Repository string `csv:"Repository"`
	Owner      string `csv:"Owner"`
	URL        string `csv:"URL"`
	IsPR       bool   `csv:"IsPR"`
	IsIssue    bool   `csv:"IsIssue"`
	Merged     bool   `csv:"Merged"`
	CreatedAt  string `csv:"CreatedAt"`
}

type PR struct {
	Title      string `csv:"Title"`
	Repository string `csv:"Repository"`
	Owner      string `csv:"Owner"`
	URL        string `csv:"URL"`
}

type result struct {
	Data struct {
		Search struct {
			IssueCount int `json:"issueCount"`
			PageInfo   struct {
				HasNextPage bool   `json:"hasNextPage"`
				EndCursor   string `json:"endCursor"`
			} `json:"pageInfo"`
			Edges []struct {
				Node struct {
					Number     int    `json:"number"`
					Title      string `json:"title"`
					Repository struct {
						NameWithOwner string `json:"nameWithOwner"`
						HomepageURL   string `json:"homepageUrl"`
						Name          string `json:"name"`
						Owner         struct {
							Name  string `json:"name"`
							Login string `json:"login"`
						} `json:"owner"`
					} `json:"repository"`
					URL       string    `json:"url"`
					CreatedAt time.Time `json:"createdAt"`
					Merged    bool      `json:"merged"`
				} `json:"node"`
			} `json:"edges"`
		} `json:"search"`
	} `json:"data"`
}

func main() {
	query := `
{
  search(query: "is:public created:2021-01-01..2021-12-31 archived:false  author:naveensrinivasan -user:naveensrinivasan", type: ISSUE, first: 100 %s) {
    issueCount
    pageInfo {
      hasNextPage
      endCursor
    }
    edges {
      node {
        ... on Issue{
          number
          title
          repository {
            nameWithOwner
            homepageUrl
            name
            owner {
              ... on Organization {
                name
                login
              }
            }
          }
          url
          createdAt
        }
        ... on PullRequest {
          number
          title
          repository {
            nameWithOwner
            homepageUrl
            name
            owner {
              ... on Organization {
                name
                login
              }
            }
          }
          url
          createdAt
          merged
        }
      }
    }
  }
}

`
	prs := `
{
  search(query: "is:public  reviewed-by:naveensrinivasan created:2021-01-01..2021-12-31 archived:false  user:ossf", type: ISSUE, first: 100 %s) {
    issueCount
    pageInfo {
      hasNextPage
      endCursor
    }
    edges {
      node {
        ... on Issue {
          number
          title
          repository {
            nameWithOwner
            homepageUrl
            name
            owner {
              ... on Organization {
                name
                login
              }
            }
          }
          url
          createdAt
        }
        ... on PullRequest {
          number
          title
          repository {
            nameWithOwner
            homepageUrl
            name
            owner {
              ... on Organization {
                name
                login
              }
            }
          }
          url
          createdAt
          mergedAt
        }
      }
    }
  }
}


`
	clientsFile, err := os.OpenFile("github.csv", os.O_RDWR|os.O_CREATE, os.ModePerm)
	if err != nil {
		panic(err)
	}
	defer clientsFile.Close()

	l := []response{}
	r := getResponse(fmt.Sprintf(query, ""), os.Getenv("GITHUB_TOKEN"))
	for r.Data.Search.PageInfo.HasNextPage {
		for _, item := range r.Data.Search.Edges {
			l = append(l, response{
				ID:         item.Node.Number,
				Title:      item.Node.Title,
				URL:        item.Node.URL,
				IsPR:       strings.Contains(item.Node.URL, "pull"),
				IsIssue:    strings.Contains(item.Node.URL, "issue"),
				Owner:      item.Node.Repository.Owner.Login,
				Repository: item.Node.Repository.Name,
				Merged:     item.Node.Merged,
				CreatedAt:  item.Node.CreatedAt.Format("2006-01-02 15:04:05"),
			})
		}
		after := fmt.Sprintf(", after:\"%s\"", r.Data.Search.PageInfo.EndCursor)
		r = getResponse(fmt.Sprintf(query, after), os.Getenv("GITHUB_TOKEN"))
		// the last one
		if !r.Data.Search.PageInfo.HasNextPage {
			for _, item := range r.Data.Search.Edges {
				l = append(l, response{
					ID:         item.Node.Number,
					Title:      item.Node.Title,
					URL:        item.Node.URL,
					IsPR:       strings.Contains(item.Node.URL, "pull"),
					IsIssue:    strings.Contains(item.Node.URL, "issue"),
					Owner:      item.Node.Repository.Owner.Login,
					Repository: item.Node.Repository.Name,
					Merged:     item.Node.Merged,
					CreatedAt:  item.Node.CreatedAt.Format("2006-01-02 15:04:05"),
				})
			}
		}
	}
	err = gocsv.MarshalFile(&l, clientsFile) // Use this to save the CSV back to the file
	if err != nil {
		panic(err)
	}
	reviews, err := os.OpenFile("reviews.csv", os.O_RDWR|os.O_CREATE, os.ModePerm)
	if err != nil {
		panic(err)
	}
	defer reviews.Close()

	prList := []PR{}
	r = getResponse(fmt.Sprintf(prs, ""), os.Getenv("GITHUB_TOKEN"))
	for r.Data.Search.PageInfo.HasNextPage {
		for _, item := range r.Data.Search.Edges {
			prList = append(prList, PR{
				Title:      item.Node.Title,
				URL:        item.Node.URL,
				Owner:      item.Node.Repository.Owner.Login,
				Repository: item.Node.Repository.Name,
			})
		}
		after := fmt.Sprintf(", after:\"%s\"", r.Data.Search.PageInfo.EndCursor)
		r = getResponse(fmt.Sprintf(query, after), os.Getenv("GITHUB_TOKEN"))
		// the last one
		if !r.Data.Search.PageInfo.HasNextPage {
			for _, item := range r.Data.Search.Edges {
				prList = append(prList, PR{
					Title:      item.Node.Title,
					URL:        item.Node.URL,
					Owner:      item.Node.Repository.Owner.Login,
					Repository: item.Node.Repository.Name,
				})
			}
		}
	}
	err = gocsv.MarshalFile(&prList, reviews) // Use this to save the CSV back to the file
	if err != nil {
		panic(err)
	}
}

func getResponse(q, token string) result {
	type Payload struct {
		Query string `json:"query"`
	}

	data := Payload{
		Query: q,
	}
	payloadBytes, err := json.Marshal(data)
	if err != nil {
		fmt.Println(err)
	}
	body := bytes.NewReader(payloadBytes)

	req, err := http.NewRequest("POST", "https://api.github.com/graphql", body)
	if err != nil {
		fmt.Println(err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("bearer %s", token))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Println(err)
	}
	defer resp.Body.Close()
	x := &result{}
	json.NewDecoder(resp.Body).Decode(x)
	return *x
}
