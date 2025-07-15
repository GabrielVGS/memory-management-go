package main

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
)

const PAGE_SIZE = 4096 // 4KB

type PageAccess struct {
	PageID string
	Type   string // "I" para instrução, "D" para dados
}

type PageFrame struct {
	PageID     string
	Referenced bool
	LoadCount  int
}

type Simulator struct {
	memorySize    int
	totalFrames   int
	accesses      []PageAccess
	distinctPages map[string]bool
	pageLoadCount map[string]int
	didacticMode  bool
	showLoadCount bool
	showPageTable bool
	skipOptimal   bool
}

func NewSimulator(memorySize int) *Simulator {
	return &Simulator{
		memorySize:    memorySize,
		totalFrames:   memorySize / PAGE_SIZE,
		distinctPages: make(map[string]bool),
		pageLoadCount: make(map[string]int),
		didacticMode:  false,
		showLoadCount: false,
		showPageTable: false,
	}
}

func (s *Simulator) LoadAccessFile(filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("erro ao abrir arquivo %s: %v", filename, err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineCount := 0
	invalidLines := 0

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		lineCount++

		if line == "" {
			continue
		}

		parts := strings.Fields(line)
		var pageID string

		if len(parts) >= 2 {
			// Formato: "numero_linha página" (ex: "1 I0")
			pageID = parts[1]
		} else if len(parts) == 1 {
			// Formato: apenas "página" (ex: "I0")
			pageID = parts[0]
		} else {
			invalidLines++
			if invalidLines <= 10 { // Mostra apenas as primeiras 10 linhas inválidas
				fmt.Printf("Aviso: Linha %d ignorada (formato inválido): %s\n", lineCount, line)
			}
			continue
		}

		// Verifica se o pageID tem formato válido (começa com I ou D)
		if len(pageID) >= 2 && (pageID[0] == 'I' || pageID[0] == 'D') {
			pageAccess := PageAccess{
				PageID: pageID,
				Type:   string(pageID[0]), // Primeiro caractere (I ou D)
			}
			s.accesses = append(s.accesses, pageAccess)
			s.distinctPages[pageAccess.PageID] = true
		} else {
			invalidLines++
			if invalidLines <= 10 {
				fmt.Printf("Aviso: Linha %d ignorada (formato de página inválido): %s\n", lineCount, line)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("erro ao ler arquivo: %v", err)
	}

	if invalidLines > 10 {
		fmt.Printf("... e mais %d linhas inválidas (não mostradas)\n", invalidLines-10)
	}

	if len(s.accesses) == 0 {
		return fmt.Errorf("nenhum acesso válido encontrado no arquivo")
	}

	fmt.Printf("Arquivo processado: %d linhas lidas, %d acessos válidos, %d linhas inválidas\n",
		lineCount, len(s.accesses), invalidLines)

	return nil
}

// Algoritmo Ótimo (OPT) - Versão Otimizada
func (s *Simulator) OptimalAlgorithm() int {
	frames := make([]string, 0, s.totalFrames)
	inMemory := make(map[string]bool) // mapa para verificar a existência de uma página no frame em O(1)
	pageFaults := 0

	s.pageLoadCount = make(map[string]int)

	// Pre-calcula as posições de todos os acessos futuros para cada página.
	// Isso evita a necessidade de escanear a lista de acessos repetidamente.
	pageAccessPositions := make(map[string][]int)
	for i, access := range s.accesses {
		pageAccessPositions[access.PageID] = append(pageAccessPositions[access.PageID], i)
	}

	for i, access := range s.accesses {
		pageID := access.PageID

		// Verifica se a página já está na memória
		if _, found := inMemory[pageID]; found {
			if s.didacticMode {
				fmt.Printf("Acesso %d - Página %s: Hit\n", i+1, pageID)
			}
			continue
		}

		// Falta de página
		pageFaults++
		s.pageLoadCount[pageID]++

		if len(frames) < s.totalFrames {
			// Ainda há espaço na memória
			frames = append(frames, pageID)
			inMemory[pageID] = true
		} else {
			// Precisa substituir uma página
			// Encontra a página que será usada mais tarde (ou nunca mais)
			farthest := -1
			victimIndex := -1

			for frameIdx, pageInFrame := range frames {
				// Procura o próximo uso desta página usando os dados pré-calculados
				positions := pageAccessPositions[pageInFrame]

				// Usa busca binária (sort.SearchInts) para encontrar eficientemente a próxima ocorrência
				nextUsePos := sort.SearchInts(positions, i+1)

				if nextUsePos == len(positions) {
					// Esta página nunca mais será usada, é a vítima ideal.
					victimIndex = frameIdx
					break // Encontramos a melhor vítima, podemos parar a busca.
				}

				nextUse := positions[nextUsePos]
				if nextUse > farthest {
					farthest = nextUse
					victimIndex = frameIdx
				}
			}

			// Remove a página vítima da memória
			delete(inMemory, frames[victimIndex])
			// Adiciona a nova página
			frames[victimIndex] = pageID
			inMemory[pageID] = true
		}

		if s.didacticMode {
			fmt.Printf("Acesso %d - Página %s: Falta de página\n", i+1, pageID)
			fmt.Printf("Estado da memória: %v\n", frames)
			fmt.Println("---")
		}
	}

	return pageFaults
}

// Algoritmo do Relógio (Clock)
func (s *Simulator) ClockAlgorithm() int {
	frames := make([]*PageFrame, s.totalFrames)
	pageToFrame := make(map[string]int)
	clockPointer := 0
	pageFaults := 0

	s.pageLoadCount = make(map[string]int)

	for i, access := range s.accesses {
		pageID := access.PageID

		// Verifica se a página já está na memória
		if frameIndex, exists := pageToFrame[pageID]; exists {
			// Hit - marca como referenciada
			frames[frameIndex].Referenced = true
			if s.didacticMode {
				fmt.Printf("Acesso %d - Página %s: Hit\n", i+1, pageID)
			}
			continue
		}

		// Falta de página
		pageFaults++
		s.pageLoadCount[pageID]++

		// Procura por um frame vazio primeiro
		emptyFrame := -1
		for j := 0; j < s.totalFrames; j++ {
			if frames[j] == nil {
				emptyFrame = j
				break
			}
		}

		if emptyFrame != -1 {
			// Usa frame vazio
			frames[emptyFrame] = &PageFrame{
				PageID:     pageID,
				Referenced: true,
				LoadCount:  1,
			}
			pageToFrame[pageID] = emptyFrame
		} else {
			// Usa algoritmo do relógio para encontrar vítima
			for {
				if !frames[clockPointer].Referenced {
					// Encontrou vítima
					oldPageID := frames[clockPointer].PageID
					delete(pageToFrame, oldPageID)

					frames[clockPointer] = &PageFrame{
						PageID:     pageID,
						Referenced: true,
						LoadCount:  1,
					}
					pageToFrame[pageID] = clockPointer
					clockPointer = (clockPointer + 1) % s.totalFrames
					break
				} else {
					// Dá segunda chance
					frames[clockPointer].Referenced = false
					clockPointer = (clockPointer + 1) % s.totalFrames
				}
			}
		}

		if s.didacticMode {
			fmt.Printf("Acesso %d - Página %s: Falta de página\n", i+1, pageID)
			s.printMemoryState(frames)
			fmt.Println("---")
		}
	}

	return pageFaults
}

func (s *Simulator) printMemoryState(frames []*PageFrame) {
	fmt.Print("Estado da memória: [")
	for i, frame := range frames {
		if frame != nil {
			refChar := "R"
			if !frame.Referenced {
				refChar = "NR"
			}
			fmt.Printf("%s(%s)", frame.PageID, refChar)
		} else {
			fmt.Print("vazio")
		}
		if i < len(frames)-1 {
			fmt.Print(", ")
		}
	}
	fmt.Println("]")
}

func (s *Simulator) ShowLoadCount() {
	if !s.showLoadCount {
		return
	}

	fmt.Println("\n=== NÚMERO DE CARREGAMENTOS POR PÁGINA ===")

	// Ordena as páginas para exibição organizada
	var pages []string
	for page := range s.pageLoadCount {
		pages = append(pages, page)
	}
	sort.Strings(pages)

	for _, page := range pages {
		fmt.Printf("Página %s: %d carregamentos\n", page, s.pageLoadCount[page])
	}
}

func (s *Simulator) EstimatePageTableSize() {
	if !s.showPageTable {
		return
	}

	fmt.Println("\n=== ESTIMATIVA DO TAMANHO DA TABELA DE PÁGINAS ===")

	// Cada entrada da tabela de páginas tem tipicamente 8 bytes
	entrySize := 8
	numDistinctPages := len(s.distinctPages)

	// Para uma tabela de páginas de 1 nível, precisamos de uma entrada
	// para cada página possível no espaço de endereçamento
	// Mas aqui vamos calcular baseado apenas nas páginas acessadas
	tableSize := numDistinctPages * entrySize

	fmt.Printf("Páginas distintas acessadas: %d\n", numDistinctPages)
	fmt.Printf("Tamanho por entrada: %d bytes\n", entrySize)
	fmt.Printf("Tamanho estimado da tabela: %d bytes (%.2f KB)\n",
		tableSize, float64(tableSize)/1024.0)
}

func (s *Simulator) Run() {
	fmt.Println("=== SIMULADOR DE PAGINAÇÃO ===")
	fmt.Printf("Tamanho da memória física: %d bytes (%.2f MB)\n",
		s.memorySize, float64(s.memorySize)/(1024*1024))
	fmt.Printf("Tamanho da página: %d bytes\n", PAGE_SIZE)
	fmt.Printf("Número de frames: %d\n", s.totalFrames)
	fmt.Printf("Número de acessos: %d\n", len(s.accesses))
	fmt.Printf("Páginas distintas: %d\n", len(s.distinctPages))

	// Estimativa de tempo
	estimatedTime := s.estimateExecutionTime()
	fmt.Printf("Tempo estimado: %s\n", estimatedTime)
	fmt.Println()

	// Verifica se há memória suficiente
	if s.totalFrames == 0 {
		fmt.Printf("ERRO: Memória insuficiente! Tamanho mínimo necessário: %d bytes (1 página)\n", PAGE_SIZE)
		return
	}

	var optimalFaults int

	// Executa algoritmo Ótimo
	if !s.skipOptimal {
		fmt.Println("=== ALGORITMO ÓTIMO ===")
		optimalFaults = s.OptimalAlgorithm()
		fmt.Printf("Faltas de página (Ótimo): %d\n", optimalFaults)
	} else {
		fmt.Println("=== ALGORITMO ÓTIMO ===")
		fmt.Println("Algoritmo ótimo ignorado (use -skipoptimal para casos extremos)")
		optimalFaults = -1 // Indica que não foi executado
	}

	// Executa algoritmo do Relógio
	fmt.Println("\n=== ALGORITMO DO RELÓGIO ===")
	clockFaults := s.ClockAlgorithm()
	fmt.Printf("Faltas de página (Relógio): %d\n", clockFaults)

	// Calcula eficiência
	if optimalFaults > 0 && clockFaults > 0 {
		efficiency := float64(optimalFaults) / float64(clockFaults) * 100
		fmt.Printf("Eficiência do algoritmo do Relógio: %.2f%%\n", efficiency)
	} else if optimalFaults == -1 {
		fmt.Println("Eficiência do algoritmo do Relógio: N/A (algoritmo ótimo não executado)")
	} else {
		fmt.Println("Eficiência do algoritmo do Relógio: N/A (sem faltas de página)")
	}

	// Mostra informações adicionais se solicitado
	s.ShowLoadCount()
	s.EstimatePageTableSize()
}

func (s *Simulator) estimateExecutionTime() string {
	// A estimativa agora pode ser mais otimista para o algoritmo ótimo
	accesses := len(s.accesses)
	if s.skipOptimal {
		return "< 5 segundos"
	}
	if accesses < 1000000 {
		return "1-5 segundos"
	} else if accesses < 10000000 {
		return "5-20 segundos"
	} else {
		return "Pode demorar mais de 30 segundos"
	}
}

func main() {
	if len(os.Args) < 3 {
		fmt.Println("Uso: go run main.go <arquivo_entrada> <tamanho_memoria_bytes> [opções]")
		fmt.Println("Opções:")
		fmt.Println("  -didactic     : Modo didático (mostra estado da memória)")
		fmt.Println("  -loadcount    : Mostra número de carregamentos por página")
		fmt.Println("  -pagetable    : Mostra estimativa do tamanho da tabela de páginas")
		fmt.Println("  -skipoptimal  : Pula algoritmo ótimo (para arquivos muito grandes)")
		fmt.Println()
		fmt.Println("Exemplos de tamanho de memória:")
		fmt.Println("  8192          : 8 KB")
		fmt.Println("  32768         : 32 KB")
		fmt.Println("  16777216      : 16 MB")
		fmt.Println("  134217728     : 128 MB")
		fmt.Println("  1073741824    : 1 GB")
		fmt.Println()
		fmt.Printf("NOTA: Tamanho mínimo de memória deve ser pelo menos %d bytes (1 página)\n", PAGE_SIZE)
		return
	}

	filename := os.Args[1]
	memorySize, err := strconv.Atoi(os.Args[2])
	if err != nil {
		fmt.Printf("Erro: tamanho de memória inválido: %s\n", os.Args[2])
		return
	}

	if memorySize < PAGE_SIZE {
		fmt.Printf("Erro: tamanho de memória muito pequeno (%d bytes).\n", memorySize)
		fmt.Printf("Tamanho mínimo necessário: %d bytes (1 página de 4KB)\n", PAGE_SIZE)
		return
	}

	simulator := NewSimulator(memorySize)

	// Processa opções
	for i := 3; i < len(os.Args); i++ {
		switch os.Args[i] {
		case "-all":
			simulator.didacticMode = true
			simulator.showLoadCount = true
			simulator.showPageTable = true
			break
		case "-didactic":
			simulator.didacticMode = true
		case "-loadcount":
			simulator.showLoadCount = true
		case "-pagetable":
			simulator.showPageTable = true
		case "-skipoptimal":
			simulator.skipOptimal = true
		default:
			fmt.Printf("Opção desconhecida: %s\n", os.Args[i])
		}
	}

	// Carrega arquivo de entrada
	fmt.Printf("Carregando arquivo: %s\n", filename)
	err = simulator.LoadAccessFile(filename)
	if err != nil {
		fmt.Printf("Erro ao carregar arquivo: %v\n", err)
		return
	}

	fmt.Printf("Arquivo carregado com sucesso!\n\n")

	// Executa simulação
	simulator.Run()
}
