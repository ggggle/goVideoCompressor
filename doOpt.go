package main

import (
    "github.com/digitalocean/godo"
    "github.com/digitalocean/godo/context"
    "golang.org/x/oauth2"
    "fmt"
    "strconv"
    "golang.org/x/crypto/ssh"
    "io/ioutil"
    "os"
    "net"
)

const dockerSlugName = "docker-16-04"

type TokenSource struct {
    AccessToken string
}

func (t *TokenSource) Token() (*oauth2.Token, error) {
    token := &oauth2.Token{
        AccessToken: t.AccessToken,
    }
    return token, nil
}

type DockerInfo struct {
    IP string
    ID int
}

type DigitalOceanApi struct {
    accessToken    string
    client         *godo.Client
    ctx            context.Context
    sshFingerprint string
}

func NewAPI(token string) *DigitalOceanApi {
    api := new(DigitalOceanApi)
    tokenSource := &TokenSource{
        AccessToken: token,
    }
    oauthClient := oauth2.NewClient(context.Background(), tokenSource)
    api.client = godo.NewClient(oauthClient)
    api.ctx = context.TODO()
    api.sshFingerprint = api.GetFirstSSHKeyFingerprint()
    return api
}

func (api *DigitalOceanApi) CreateDocker(num int) (droplet []godo.Droplet) {
    var names []string
    for i := 0; i < num; i++ {
        names = append(names, "docker"+strconv.Itoa(i))
    }
    fmt.Println(names)
    createRequest := &godo.DropletMultiCreateRequest{
        Names:  names,
        Region: "sfo2",
        Size:   "s-3vcpu-1gb",
        Image: godo.DropletCreateImage{
            Slug: dockerSlugName,
        },
        SSHKeys: []godo.DropletCreateSSHKey{
            {
                Fingerprint: api.sshFingerprint,
            },
        },
        IPv6: true,
        Tags: []string{"web"},
    }
    droplet, _, err := api.client.Droplets.CreateMultiple(api.ctx, createRequest)
    if err != nil {
        fmt.Print(err)
    }
    return
}

func (api *DigitalOceanApi) ListAllDroplet() (droplets []godo.Droplet) {
    opt := &godo.ListOptions{
        Page:    1,
        PerPage: 200,
    }
    droplets, _, err := api.client.Droplets.List(api.ctx, opt)
    if (err != nil) {
        fmt.Print(err)
    }
    return
}

func (api *DigitalOceanApi) DeleteAllDocker() (deleteNum int) {
    droplets := api.ListAllDroplet()
    for _, value := range droplets {
        if value.Image.Slug == dockerSlugName {
            api.client.Droplets.Delete(api.ctx, value.ID)
            deleteNum++
        }
    }
    return
}

func (api *DigitalOceanApi) GetAllDockerIP() (IP []string) {
    droplets := api.ListAllDroplet()
    for _, value := range droplets {
        if value.Image.Slug == dockerSlugName {
            IP = append(IP, value.Networks.V4[0].IPAddress)
        }
    }
    return
}

func (api *DigitalOceanApi) GetAllSSHKey() (sshKey []godo.Key) {
    opt := &godo.ListOptions{
        Page:    1,
        PerPage: 200,
    }
    sshKey, _, err := api.client.Keys.List(api.ctx, opt)
    if err != nil {
        fmt.Print(err)
    }
    return
}

func (api *DigitalOceanApi) GetFirstSSHKeyFingerprint() (fingerprint string) {
    allKey := api.GetAllSSHKey()
    if len(allKey) > 0 {
        fingerprint = allKey[0].Fingerprint
    }
    return
}

type SSHClient struct {
    IP      string
    session *ssh.Session
}

func NewSSH(IP, rsaKey string) *SSHClient {
    b, err := ioutil.ReadFile(rsaKey)
    if err != nil {
        fmt.Println(err)
        return nil
    }
    pKey, err := ssh.ParsePrivateKey(b)
    if err != nil {
        fmt.Println(err)
        return nil
    }
    config := ssh.ClientConfig{
        User: "root",
        Auth: []ssh.AuthMethod{
            ssh.PublicKeys(pKey),
        },
        HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
            return nil
        },
    }
    client, err := ssh.Dial("tcp", IP+":22", &config)
    if err != nil {
        fmt.Println(err)
        return nil
    }
    var sshClient *SSHClient = new(SSHClient)
    sshClient.IP = IP
    sshClient.session, err = client.NewSession()
    if err != nil {
        fmt.Println(err)
        return nil
    }
    sshClient.session.Stdout = os.Stdout
    sshClient.session.Stderr = os.Stderr
    return sshClient
}

func (ssh *SSHClient) Exec(cmd string) {
    if ssh.session == nil {
        fmt.Println("session为空")
        return
    }
    ssh.session.Run(cmd)
}

func main() {
    api := NewAPI("")
    api.DeleteAllDocker()
    /*
    for _, value := range api.GetAllDockerIP() {
        sshc := NewSSH(value, "do2")
        sshc.Exec("docker run -d hey678/myclient")
    }
    num := api.DeleteAllDocker()
    fmt.Print(num)*/
}
