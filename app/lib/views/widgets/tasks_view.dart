import 'package:flutter/material.dart';

class TasksView extends StatefulWidget {
  final List<String> tasks;
  final String name;
  final Function onImplementPlanButtonPressed;
  final Function onRequestNewPlanButtonPressed;
  final Function onBackButtonPressed;

  const TasksView(
      {super.key,
      required this.tasks,
      required this.name,
      required this.onImplementPlanButtonPressed,
      required this.onRequestNewPlanButtonPressed,
      required this.onBackButtonPressed});

  @override
  TasksViewState createState() => TasksViewState();
}

class TasksViewState extends State<TasksView>
    with SingleTickerProviderStateMixin {
  List<String> _tasks = [];
  final TextEditingController _newTaskController = TextEditingController();
  final FocusNode _inputFocusNode = FocusNode();
  final ScrollController _scrollController = ScrollController();
  int? _selectedTaskIndex;
  late AnimationController _delayController;

  @override
  void initState() {
    super.initState();
    _delayController = AnimationController(
      vsync: this,
      duration: const Duration(milliseconds: 100),
    );
    _tasks = widget.tasks;
  }

  @override
  void dispose() {
    _delayController.dispose();
    _newTaskController.dispose();
    _inputFocusNode.dispose();
    _scrollController.dispose();
    super.dispose();
  }

  Widget _buildTaskItem(int index) {
    if (index == _selectedTaskIndex) {
      return TextFormField(
        initialValue: _tasks[index],
        autofocus: true,
        onFieldSubmitted: (value) {
          setState(() {
            _tasks[index] = value;
            _selectedTaskIndex = null;
          });
        },
      );
    } else {
      return InkWell(
        onTap: () {
          setState(() {
            _selectedTaskIndex = index;
          });
        },
        child: Text(_tasks[index]),
      );
    }
  }

  void _confirmDeletion(BuildContext context, int index) {
    showDialog(
      context: context,
      builder: (BuildContext context) {
        return AlertDialog(
          title: const Text('Confirm Deletion'),
          content: const Text('Are you sure you want to delete this task?'),
          actions: <Widget>[
            TextButton(
              child: const Text('Cancel'),
              onPressed: () {
                Navigator.of(context).pop(); // Dismiss the dialog
              },
            ),
            TextButton(
              child: const Text('Delete'),
              onPressed: () {
                setState(() {
                  _tasks.removeAt(index);
                });
                Navigator.of(context).pop(); // Dismiss the dialog
              },
            ),
          ],
        );
      },
    );
  }

  Widget _buildTaskList() {
    return ListView.separated(
      controller: _scrollController,
      itemCount: _tasks.length,
      separatorBuilder: (context, index) => const Divider(),
      itemBuilder: (context, index) {
        return ListTile(
          title: Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text('${index + 1}. '),
              Expanded(child: _buildTaskItem(index)),
              IconButton(
                icon: const Icon(Icons.delete),
                onPressed: () => _confirmDeletion(context, index),
              ),
            ],
          ),
        );
      },
    );
  }

  Widget _buildAddTaskForm() {
    return TextFormField(
        controller: _newTaskController,
        focusNode: _inputFocusNode,
        decoration: const InputDecoration(labelText: 'Add a new task'),
        onFieldSubmitted: (value) {
          setState(() {
            _tasks.add(value);
            _newTaskController.clear();
            _inputFocusNode.requestFocus();
          });
          _delayController.forward(from: 0).then((_) {
            _scrollController.animateTo(
              _scrollController.position.maxScrollExtent,
              duration: const Duration(milliseconds: 500),
              curve: Curves.easeOut,
            );
          });
        });
  }

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Padding(
          padding: const EdgeInsets.all(8.0),
          child: Text(
            widget.name == 'Unnamed Plan'
                ? widget.name
                : 'Plan to ${widget.name}',
            style: const TextStyle(fontSize: 24, fontWeight: FontWeight.bold),
          ),
        ),
        Expanded(child: _buildTaskList()),
        _buildAddTaskForm(),
        Row(
          mainAxisAlignment: MainAxisAlignment.end,
          children: [
            TextButton(
              child: const Text('Return to Chat'),
              onPressed: () {
                widget.onBackButtonPressed();
              },
            ),
            TextButton(
              child: const Text('Request New Plan'),
              onPressed: () {
                widget.onRequestNewPlanButtonPressed();
              },
            ),
            ElevatedButton(
              child: const Text('Implement Plan',
                  style: TextStyle(color: Colors.white)),
              onPressed: () {
                widget.onImplementPlanButtonPressed();
              },
            ),
          ],
        ),
        const SizedBox(height: 8),
      ],
    );
  }
}
