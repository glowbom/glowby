import 'package:flutter/material.dart';

class TasksView extends StatefulWidget {
  final List<String> tasks;
  final String name;

  TasksView({required this.tasks, required this.name});

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
    return ListView.separated(
      itemCount: _tasks.length,
      separatorBuilder: (context, index) => Divider(),
      itemBuilder: (context, index) {
        return ListTile(
          title: Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text('${index + 1}. '),
              Expanded(child: Text(_tasks[index])),
            ],
          ),
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
        ElevatedButton(
          child: Text('Execute'),
          onPressed: () {
            // Handle generating the customized plan
          },
        ),
      ],
    );
  }
}
