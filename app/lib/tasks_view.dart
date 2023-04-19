import 'package:flutter/material.dart';

class TasksView extends StatefulWidget {
  final List<String> tasks;
  final String name;
  final Function onImplementPlanButtonPressed;
  final Function onRequestNewPlanButtonPressed;
  final Function onBackButtonPressed;

  TasksView(
      {required this.tasks,
      required this.name,
      required this.onImplementPlanButtonPressed,
      required this.onRequestNewPlanButtonPressed,
      required this.onBackButtonPressed});

  @override
  _TasksViewState createState() => _TasksViewState();
}

class _TasksViewState extends State<TasksView> {
  List<String> _tasks = [];
  final TextEditingController _newTaskController = TextEditingController();
  final FocusNode _inputFocusNode = FocusNode();
  final ScrollController _scrollController = ScrollController();
  int? _selectedTaskIndex;

  @override
  void initState() {
    super.initState();
    _tasks = widget.tasks;
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

  Widget _buildTaskList() {
    return ListView.separated(
      controller: _scrollController,
      itemCount: _tasks.length,
      separatorBuilder: (context, index) => Divider(),
      itemBuilder: (context, index) {
        return ListTile(
          title: Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text('${index + 1}. '),
              Expanded(child: _buildTaskItem(index)),
              IconButton(
                icon: Icon(Icons.delete),
                onPressed: () {
                  setState(() {
                    _tasks.removeAt(index);
                  });
                },
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
      decoration: InputDecoration(labelText: 'Add a new task'),
      onFieldSubmitted: (value) {
        setState(() {
          _tasks.add(value);
          _newTaskController.clear();
          _inputFocusNode.requestFocus();
        });
        Future.delayed(Duration(milliseconds: 100)).then((_) {
          _scrollController.animateTo(
            _scrollController.position.maxScrollExtent,
            duration: Duration(milliseconds: 500),
            curve: Curves.easeOut,
          );
        });
      },
    );
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
            style: TextStyle(fontSize: 24, fontWeight: FontWeight.bold),
          ),
        ),
        Expanded(child: _buildTaskList()),
        _buildAddTaskForm(),
        Row(
          mainAxisAlignment: MainAxisAlignment.end,
          children: [
            TextButton(
              child: Text('Return to Chat'),
              onPressed: () {
                widget.onBackButtonPressed();
              },
            ),
            TextButton(
              child: Text('Request New Plan'),
              onPressed: () {
                widget.onRequestNewPlanButtonPressed();
              },
            ),
            ElevatedButton(
              child: Text('Implement Plan'),
              onPressed: () {
                widget.onImplementPlanButtonPressed();
              },
            ),
          ],
        ),
        SizedBox(height: 8),
      ],
    );
  }
}
