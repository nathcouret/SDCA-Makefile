namespace go compilationInterface

struct Command
{
	1:string program,
	2:string arguments
}

typedef i32 int
service CompilationService
{
		int executeCommand(1:Command command),
}