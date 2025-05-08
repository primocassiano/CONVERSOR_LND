# Gerador de Endereços Aezeed (GUI)

Este é um aplicativo gráfico (GUI) desenvolvido em Go com a biblioteca Fyne para interagir com seeds mnemônicas no padrão Aezeed (compatível com LND). Ele permite gerar novas seeds, decodificar seeds existentes, derivar chaves públicas estendidas (XPUBs) e endereços Bitcoin para diferentes padrões BIP (BIP44, BIP49, BIP84, BIP86), exibir a master fingerprint da seed, verificar o uso desses endereços em fontes de blockchain (Blockstream.info ou nó local Bitcoin Core) e buscar por endereços específicos dentro da seed.

## Funcionalidades Principais

*   **Geração de Nova Seed:** Cria uma nova seed Aezeed segura com entropia aleatória e exibe o mnemônico de 24 palavras correspondente.
*   **Decodificação de Mnemônico:** Permite inserir um mnemônico Aezeed de 24 palavras existente (com passphrase opcional) para carregar a seed correspondente.
*   **Exibição da Master Fingerprint:** Mostra a master fingerprint da chave mestra (root key) da seed carregada. Esta fingerprint é essencial para importar a carteira como watch-only em softwares como Sparrow Wallet, junto com a XPUB.
*   **Exibição de XPUBs:** Mostra as chaves públicas estendidas (XPUBs) da conta padrão (0) para os caminhos de derivação BIP44, BIP49, BIP84 e BIP86.
*   **Geração de Endereços com Rolagem Infinita:** Gera e exibe lotes de endereços Bitcoin para os quatro tipos de derivação (Legacy, Nested SegWit, Native SegWit, Taproot) a partir da seed carregada. Ao clicar em "Carregar Próximos 20", os novos endereços são adicionados à lista existente, permitindo rolar por todos os endereços carregados continuamente.
*   **Alternância de Endereços (Externo/Interno):** Permite alternar a visualização entre endereços externos (change 0) e internos (change 1).
*   **Verificação de Endereços:** Conecta-se a uma fonte de blockchain selecionada (Blockstream.info ou um nó Bitcoin Core local via RPC) para verificar se os endereços gerados possuem transações ou saldo.
*   **Busca de Endereço Individual:** Permite colar um endereço Bitcoin e buscar se ele pertence à seed carregada, verificando os caminhos BIP44, BIP49, BIP84 e BIP86, tanto para change 0 quanto para change 1, até um limite de índice configurável.
*   **Interface Gráfica Amigável:** Oferece uma interface intuitiva para realizar todas as operações.
*   **Tema Escuro (Opcional):** Suporte para tema escuro através de variável de ambiente para melhor conforto visual.

## Melhorias Recentes

*   **Exibição da Master Fingerprint:** A master fingerprint da seed agora é exibida acima das XPUBs, com um botão para copiar, facilitando a importação em carteiras watch-only.
*   **Rolagem Infinita de Endereços:** Ao carregar mais endereços, eles são adicionados à lista existente em vez de substituí-la, permitindo a visualização de um grande número de endereços de forma contínua.
*   **Instruções para Tema Escuro:** Adicionadas instruções sobre como ativar o tema escuro.

## Dependências

*   **Go:** Versão 1.18 ou superior.
*   **Bibliotecas Go:** As dependências são gerenciadas pelo Go Modules (arquivos `go.mod` e `go.sum`). A principal dependência externa para a GUI é `fyne.io/fyne/v2`.
*   **Dependências do Fyne (Linux):** Para compilar e executar aplicativos Fyne no Linux, você precisará de algumas bibliotecas de desenvolvimento C e drivers gráficos. O comando de instalação varia ligeiramente dependendo da sua distribuição.

## Compilação para Linux

Siga estes passos para compilar o aplicativo em um sistema Linux:

1.  **Instalar o Go:** Se ainda não tiver o Go instalado, siga as instruções oficiais em [https://go.dev/doc/install](https://go.dev/doc/install). Verifique a instalação com `go version`.

2.  **Instalar as Dependências do Fyne:** Abra um terminal e execute o comando apropriado para sua distribuição Linux:

    *   **Debian/Ubuntu/Linux Mint:**
        ```bash
        sudo apt-get update
        sudo apt-get install gcc libgl1-mesa-dev xorg-dev
        ```
    *   **Fedora/CentOS/RHEL:**
        ```bash
        sudo dnf install gcc libX11-devel libXcursor-devel libXrandr-devel libXinerama-devel mesa-libGL-devel libXi-devel libXxf86vm-devel
        ```
    *   **Arch Linux/Manjaro:**
        ```bash
        sudo pacman -Syu gcc libx11 libxcursor libxrandr libxinerama mesa libxi libxxf86vm
        ```
    *   **openSUSE:**
        ```bash
        sudo zypper install gcc libX11-devel libXcursor-devel libXrandr-devel libXinerama-devel Mesa-libGL-devel libXi-devel libXxf86vm-devel
        ```

3.  **Navegar até o Diretório do Projeto:** Use o comando `cd` para entrar na pasta onde você descompactou os arquivos do projeto (a pasta que contém `main.go`, `go.mod`, etc.).
    ```bash
    cd /caminho/para/CONVERSOR_LND
    ```

4.  **Baixar as Dependências Go:** Execute o comando abaixo para baixar todas as bibliotecas Go necessárias listadas no `go.mod`.
    ```bash
    go mod tidy
    ```

5.  **Compilar o Aplicativo:** Use o comando `go build` para criar o executável. Você pode especificar o nome do arquivo de saída com a flag `-o`.
    ```bash
    go build -o CONVERSOR_LND main.go
    ```
    Isso criará um arquivo executável chamado `CONVERSOR_LND` (ou o nome que você especificou) no diretório atual.

## Uso

Após a compilação bem-sucedida, você pode executar o aplicativo diretamente pelo terminal:

```bash
./CONVERSOR_LND
```

A interface gráfica será iniciada, permitindo que você utilize as funcionalidades descritas acima.

**Observação sobre Nó Local (RPC):** Se você optar por usar a fonte de dados "Nó Local (RPC)", certifique-se de que seu nó Bitcoin Core esteja em execução, configurado corretamente para aceitar conexões RPC (com usuário e senha definidos no `bitcoin.conf`, se necessário) e que o `addressindex=1` (ou `addrindex=1`) esteja habilitado para a funcionalidade de verificação de saldo via `scantxoutset`.
