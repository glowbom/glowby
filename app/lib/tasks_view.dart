import 'package:flutter/material.dart';

class TasksView extends StatefulWidget {
  final List<String> tasks;

  TasksView({required this.tasks});

  @override
  _TasksViewState createState() => _TasksViewState();
}

class _TasksViewState extends State<TasksView> {
  List<String> _tasks = [];
  final TextEditingController _newTaskController = TextEditingController();

  @override
  void initState() {
    super.initState();
    _tasks = widget.tasks;
  }

  Widget _buildTaskList() {
    return ListView.builder(
      itemCount: _tasks.length,
      itemBuilder: (context, index) {
        return ListTile(
          title: Text(_tasks[index]),
          onTap: () {
            // handle task editing
          },
        );
      },
    );
  }

  Widget _buildAddTaskForm() {
    return TextFormField(
      controller: _newTaskController,
      decoration: InputDecoration(labelText: 'Add a new task'),
      onFieldSubmitted: (value) {
        setState(() {
          _tasks.add(value);
          _newTaskController.clear();
        });
      },
    );
  }

  @override
  Widget build(BuildContext context) {
    return Column(
      children: [
        Expanded(child: _buildTaskList()),
        _buildAddTaskForm(),
        ElevatedButton(
          child: Text('Execute'),
          onPressed: () {
            // Handle generating the customized plan
          },
        )
      ],
    );
  }
}
